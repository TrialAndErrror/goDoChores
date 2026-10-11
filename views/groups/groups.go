// Package groups lets users form groups whose members see each other's chore
// reminders on the home page.
package groups

import (
	"encoding/json"
	"errors"
	"goDoChores/auth"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/routes"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// MemberView is one member of the viewer's group, as shown on the group page.
type MemberView struct {
	UserID   uint
	Username string
	IsOwner  bool
	IsMe     bool
}

// InviteView is a pending invite addressed to the viewer.
type InviteView struct {
	ID        uint
	GroupName string
	InvitedBy string
}

// SentInviteView is a pending invite the group has sent.
type SentInviteView struct {
	Email string
}

// TagOption is one tag in the "Tags to Show" setting.
type TagOption struct {
	ID    uint
	Name  string
	Shown bool
}

// pageData is everything the group page renders.
type pageData struct {
	Group   *models.Group // nil when the viewer is in no group
	IsOwner bool
	Members []MemberView
	Sent    []SentInviteView
	Invites []InviteView
	Tags    []TagOption
}

// MembershipOf returns the viewer's group membership, or nil if they are in no
// group.
func MembershipOf(db *gorm.DB, userID uint) (*models.GroupMember, error) {
	var m models.GroupMember
	err := db.Where("user_id = ?", userID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// OtherMemberIDs returns the other members of the viewer's group. It is empty
// when the viewer has no group.
func OtherMemberIDs(db *gorm.DB, viewerID uint) ([]uint, error) {
	m, err := MembershipOf(db, viewerID)
	if err != nil || m == nil {
		return nil, err
	}
	var ids []uint
	err = db.Model(&models.GroupMember{}).
		Where("group_id = ? AND user_id <> ?", m.GroupID, viewerID).
		Pluck("user_id", &ids).Error
	return ids, err
}

// PendingInviteCount is how many invites are waiting for the user to answer.
func PendingInviteCount(db *gorm.DB, userID uint) (int64, error) {
	var email string
	if err := db.Model(&models.User{}).Select("email").Where("id = ?", userID).Scan(&email).Error; err != nil {
		return 0, err
	}
	var n int64
	err := db.Model(&models.GroupInvite{}).Where("email = ?", lowerEmail(email)).Count(&n).Error
	return n, err
}

func lowerEmail(email string) string {
	e, err := models.NormalizeEmail(email)
	if err != nil {
		return ""
	}
	return e
}

func currentUser(w http.ResponseWriter, r *http.Request) (models.User, bool) {
	userID, err := auth.GetCurrentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return models.User{}, false
	}
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return models.User{}, false
	}
	return user, true
}

// ownedGroup returns the group the user owns, or writes an error and false.
func ownedGroup(w http.ResponseWriter, user models.User) (models.Group, bool) {
	var g models.Group
	err := database.DB.Where("owner_id = ?", user.ID).First(&g).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.Error(w, "Only the group owner can do that", http.StatusForbidden)
		return g, false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return g, false
	}
	return g, true
}

func back(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, routes.URLFor("groups"), http.StatusSeeOther)
}

func serverError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func Page(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	db := database.DB
	data := pageData{}

	m, err := MembershipOf(db, user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	if m != nil {
		var g models.Group
		if err := db.First(&g, m.GroupID).Error; err != nil {
			serverError(w, err)
			return
		}
		data.Group = &g
		data.IsOwner = g.OwnerID == user.ID

		var members []models.GroupMember
		if err := db.Preload("User").Where("group_id = ?", g.ID).Order("id").Find(&members).Error; err != nil {
			serverError(w, err)
			return
		}
		for _, gm := range members {
			data.Members = append(data.Members, MemberView{
				UserID:   gm.UserID,
				Username: gm.User.Username,
				IsOwner:  gm.UserID == g.OwnerID,
				IsMe:     gm.UserID == user.ID,
			})
		}

		if data.IsOwner {
			var sent []models.GroupInvite
			if err := db.Where("group_id = ?", g.ID).Order("id").Find(&sent).Error; err != nil {
				serverError(w, err)
				return
			}
			for _, inv := range sent {
				data.Sent = append(data.Sent, SentInviteView{Email: inv.Email})
			}
		}
	}

	var invites []models.GroupInvite
	if err := db.Preload("Group").Preload("InvitedBy").Where("email = ?", lowerEmail(user.Email)).Order("id").Find(&invites).Error; err != nil {
		serverError(w, err)
		return
	}
	for _, inv := range invites {
		data.Invites = append(data.Invites, InviteView{ID: inv.ID, GroupName: inv.Group.Name, InvitedBy: inv.InvitedBy.Username})
	}

	tags, err := TagOptions(db, user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	data.Tags = tags

	if err := groupPage(data).Render(r.Context(), w); err != nil {
		serverError(w, err)
	}
}

func Create(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name, err := models.NormalizeGroupName(r.PostForm.Get("name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		g := models.Group{Name: name, OwnerID: user.ID}
		if err := tx.Create(&g).Error; err != nil {
			return err
		}
		// The unique index on user_id fails this if they are already in a group.
		if err := tx.Create(&models.GroupMember{GroupID: g.ID, UserID: user.ID}).Error; err != nil {
			return err
		}
		return models.MoveTagsToGroup(tx, user.ID, g.ID)
	})
	if err != nil {
		if m, _ := MembershipOf(database.DB, user.ID); m != nil {
			http.Error(w, "You are already in a group", http.StatusConflict)
			return
		}
		serverError(w, err)
		return
	}
	back(w, r)
}

func Rename(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	g, ok := ownedGroup(w, user)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name, err := models.NormalizeGroupName(r.PostForm.Get("name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := database.DB.Model(&g).Update("name", name).Error; err != nil {
		serverError(w, err)
		return
	}
	back(w, r)
}

func Invite(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	g, ok := ownedGroup(w, user)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	email, err := models.NormalizeEmail(r.PostForm.Get("email"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	db := database.DB

	// Skip people already in the group. Whether the address belongs to an
	// account at all is deliberately not revealed: the invite is simply stored
	// and shows up if and when a user with that email exists.
	var already int64
	if err := db.Model(&models.GroupMember{}).
		Joins("join users on users.id = group_members.user_id").
		Where("group_members.group_id = ? AND lower(users.email) = ?", g.ID, email).
		Count(&already).Error; err != nil {
		serverError(w, err)
		return
	}
	if already > 0 {
		http.Error(w, "That person is already in your group", http.StatusConflict)
		return
	}

	inv := models.GroupInvite{GroupID: g.ID, Email: email, InvitedByID: user.ID}
	// Re-inviting someone who already has a pending invite is a no-op.
	if err := db.Where("group_id = ? AND email = ?", g.ID, email).FirstOrCreate(&inv).Error; err != nil {
		serverError(w, err)
		return
	}
	back(w, r)
}

func AnswerInvite(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	answer := chi.URLParam(r, "answer")
	if answer != "accept" && answer != "reject" {
		http.NotFound(w, r)
		return
	}
	inviteID, err := strconv.ParseUint(chi.URLParam(r, "inviteID"), 10, 32)
	if err != nil {
		http.Error(w, "Invalid invite ID", http.StatusBadRequest)
		return
	}
	db := database.DB

	var inv models.GroupInvite
	// An invite is only visible to the address it was sent to.
	if err := db.Where("email = ?", lowerEmail(user.Email)).First(&inv, inviteID).Error; err != nil {
		http.Error(w, "Invite not found", http.StatusNotFound)
		return
	}

	if answer == "reject" {
		if err := db.Delete(&inv).Error; err != nil {
			serverError(w, err)
			return
		}
		back(w, r)
		return
	}

	if m, err := MembershipOf(db, user.ID); err != nil {
		serverError(w, err)
		return
	} else if m != nil {
		http.Error(w, "Leave your current group before accepting an invite", http.StatusConflict)
		return
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&models.GroupMember{GroupID: inv.GroupID, UserID: user.ID}).Error; err != nil {
			return err
		}
		if err := models.MoveTagsToGroup(tx, user.ID, inv.GroupID); err != nil {
			return err
		}
		return tx.Where("group_id = ? AND email = ?", inv.GroupID, inv.Email).Delete(&models.GroupInvite{}).Error
	})
	if err != nil {
		// A double-click or a second invite accepted at the same moment loses
		// to the unique index on user_id.
		if m, _ := MembershipOf(db, user.ID); m != nil {
			http.Error(w, "You are already in a group", http.StatusConflict)
			return
		}
		serverError(w, err)
		return
	}
	back(w, r)
}

func Leave(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	db := database.DB
	m, err := MembershipOf(db, user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	if m == nil {
		back(w, r)
		return
	}
	var g models.Group
	if err := db.First(&g, m.GroupID).Error; err != nil {
		serverError(w, err)
		return
	}
	if g.OwnerID == user.ID {
		http.Error(w, "The owner cannot leave; delete the group instead", http.StatusConflict)
		return
	}
	// Scoped to this group and user so a concurrent change can't remove a
	// different membership. The group's tags stay behind for the others, and
	// the user keeps their own copies of the ones their chores use.
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ? AND user_id = ?", g.ID, user.ID).Delete(&models.GroupMember{}).Error; err != nil {
			return err
		}
		return models.ForkTagsToOwner(tx, user.ID, g.ID)
	})
	if err != nil {
		serverError(w, err)
		return
	}
	back(w, r)
}

func Delete(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	g, ok := ownedGroup(w, user)
	if !ok {
		return
	}
	if err := models.DeleteGroup(database.DB, g.ID); err != nil {
		serverError(w, err)
		return
	}
	back(w, r)
}

// GroupTags returns the tags of the user's group, by name. It is empty when
// the user is in no group.
func GroupTags(db *gorm.DB, userID uint) ([]models.Tag, error) {
	m, err := MembershipOf(db, userID)
	if err != nil || m == nil {
		return nil, err
	}
	var tags []models.Tag
	err = db.Where("group_id = ?", m.GroupID).Order("name").Find(&tags).Error
	return tags, err
}

// UserTags returns the tags the user can put on chores, by name: their group's
// tags when they are in a group, otherwise their personal ones.
func UserTags(db *gorm.DB, userID uint) ([]models.Tag, error) {
	m, err := MembershipOf(db, userID)
	if err != nil {
		return nil, err
	}
	var tags []models.Tag
	q := db.Order("name")
	if m != nil {
		q = q.Where("group_id = ?", m.GroupID)
	} else {
		q = q.Where("owner_id = ?", userID)
	}
	return tags, q.Find(&tags).Error
}

// HiddenTags returns the ids of the tags the user has chosen not to show.
func HiddenTags(db *gorm.DB, userID uint) (map[uint]bool, error) {
	var ids []uint
	if err := db.Model(&models.HiddenTag{}).Where("user_id = ?", userID).Pluck("tag_id", &ids).Error; err != nil {
		return nil, err
	}
	set := make(map[uint]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

// TagOptions lists the tags of the user's group, marking the ones the user
// currently shows.
func TagOptions(db *gorm.DB, userID uint) ([]TagOption, error) {
	tags, err := GroupTags(db, userID)
	if err != nil {
		return nil, err
	}
	hidden, err := HiddenTags(db, userID)
	if err != nil {
		return nil, err
	}
	opts := make([]TagOption, len(tags))
	for i, t := range tags {
		opts[i] = TagOption{ID: t.ID, Name: t.Name, Shown: !hidden[t.ID]}
	}
	return opts, nil
}

// CreateTag adds a tag for the user, in their group if they have one, and
// answers with it as JSON, so the chore form can add it to its dropdown without
// reloading. A tag that already exists is returned as is.
func CreateTag(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name, err := models.NormalizeTag(r.PostForm.Get("name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m, err := MembershipOf(database.DB, user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	var tag models.Tag
	if m != nil {
		tag, err = models.FindOrCreateTag(database.DB, &m.GroupID, nil, name)
	} else {
		tag, err = models.FindOrCreateTag(database.DB, nil, &user.ID, name)
	}
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": tag.ID, "name": tag.Name})
}

// SaveTags saves the user's "Tags to Show" setting. The form lists the id of
// every tag it displayed (all) and the ones left checked (show); the rest are
// hidden.
func SaveTags(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Only tags of the user's own group can be saved, which also bounds the
	// work a crafted form can cause.
	options, err := TagOptions(database.DB, user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	offered := make(map[string]bool, len(options))
	for _, o := range options {
		offered[strconv.FormatUint(uint64(o.ID), 10)] = true
	}
	shown := map[string]bool{}
	for _, id := range r.PostForm["show"] {
		shown[id] = true
	}
	saveErr := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, raw := range r.PostForm["all"] {
			if !offered[raw] {
				continue
			}
			id, _ := strconv.ParseUint(raw, 10, 32)
			if shown[raw] {
				if err := tx.Where("user_id = ? AND tag_id = ?", user.ID, id).Delete(&models.HiddenTag{}).Error; err != nil {
					return err
				}
				continue
			}
			h := models.HiddenTag{UserID: user.ID, TagID: uint(id)}
			if err := tx.Where("user_id = ? AND tag_id = ?", user.ID, id).FirstOrCreate(&h).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if saveErr != nil {
		serverError(w, saveErr)
		return
	}
	back(w, r)
}
