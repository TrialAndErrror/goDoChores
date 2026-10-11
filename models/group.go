package models

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
)

// MaxGroupNameLength is the longest display name a group can have.
const MaxGroupNameLength = 100

// Group is a set of users who see each other's chore reminders on the home
// page. ID is the group id shown to its members. Group rows are hard-deleted.
type Group struct {
	ID      uint `gorm:"primarykey"`
	Name    string
	OwnerID uint `gorm:"not null"`

	Members []GroupMember
}

// GroupMember puts a user in a group. UserID is unique, so a user is in at most
// one group at a time.
type GroupMember struct {
	ID      uint `gorm:"primarykey"`
	GroupID uint `gorm:"not null;index"`
	UserID  uint `gorm:"not null;uniqueIndex"`
	User    User
}

// GroupInvite is a pending invitation, addressed to an email address (stored
// lower-cased). Accepting it creates a GroupMember; declining deletes it. Either
// way the row goes away, so every row is a pending invite.
type GroupInvite struct {
	ID          uint   `gorm:"primarykey"`
	GroupID     uint   `gorm:"not null;uniqueIndex:idx_group_invite_email"`
	Email       string `gorm:"not null;uniqueIndex:idx_group_invite_email;index"`
	InvitedByID uint   `gorm:"not null"`

	Group     Group
	InvitedBy User
}

// NormalizeGroupName trims name and checks that it is usable as a group name.
func NormalizeGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("group name is required")
	}
	if utf8.RuneCountInString(name) > MaxGroupNameLength {
		return "", errors.New("group name is too long")
	}
	return name, nil
}

// NormalizeEmail returns the bare, lower-cased address in s, or an error if s
// is not a single email address.
func NormalizeEmail(s string) (string, error) {
	s = strings.TrimSpace(s)
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return "", errors.New("enter a valid email address")
	}
	return strings.ToLower(addr.Address), nil
}

const (
	// MaxTagLength is the longest a single tag can be.
	MaxTagLength = 30
	// MaxTagsPerChore bounds how many tags one chore can carry.
	MaxTagsPerChore = 10
)

// Tag is a label on chores. It lives in exactly one scope: a group's tag is
// shared by every member, and a personal tag (OwnerID set) belongs to a user who
// is in no group. Names are lower-cased, so "Kitchen" and "kitchen" are one tag.
// Tag rows are hard-deleted.
type Tag struct {
	ID      uint   `gorm:"primarykey"`
	GroupID *uint  `gorm:"uniqueIndex:idx_tag_name"`
	OwnerID *uint  `gorm:"uniqueIndex:idx_tag_owner_name"`
	Name    string `gorm:"not null;uniqueIndex:idx_tag_name;uniqueIndex:idx_tag_owner_name"`
}

// ChoreTag puts a Tag on a chore.
type ChoreTag struct {
	ID      uint `gorm:"primarykey"`
	ChoreID uint `gorm:"not null;uniqueIndex:idx_chore_tag"`
	TagID   uint `gorm:"not null;uniqueIndex:idx_chore_tag;index"`
	Tag     Tag
}

// HiddenTag is part of a user's "Tags to Show" setting: reminders for chores
// carrying this tag are not shown to that user.
type HiddenTag struct {
	ID     uint `gorm:"primarykey"`
	UserID uint `gorm:"not null;uniqueIndex:idx_hidden_tag"`
	TagID  uint `gorm:"not null;uniqueIndex:idx_hidden_tag"`
}

// NormalizeTag trims and lower-cases a single tag name and checks it.
func NormalizeTag(raw string) (string, error) {
	tag := strings.ToLower(strings.TrimSpace(raw))
	if tag == "" {
		return "", errors.New("tag is empty")
	}
	if utf8.RuneCountInString(tag) > MaxTagLength {
		return "", errors.New("tag is too long")
	}
	return tag, nil
}

// groupTagIDs is a subquery selecting the ids of a group's tags.
func groupTagIDs(tx *gorm.DB, groupID uint) *gorm.DB {
	return tx.Model(&Tag{}).Select("id").Where("group_id = ?", groupID)
}

// userChoreIDs is a subquery selecting a user's chores. It is unscoped so
// soft-deleted chores keep no dangling tag links either.
func userChoreIDs(tx *gorm.DB, userID uint) *gorm.DB {
	return tx.Unscoped().Model(&Chore{}).Select("id").Where("user_id = ?", userID)
}

// FindOrCreateTag returns the tag with this name in the given scope, creating it
// if needed. Exactly one of groupID and ownerID must be set.
func FindOrCreateTag(tx *gorm.DB, groupID, ownerID *uint, name string) (Tag, error) {
	tag := Tag{GroupID: groupID, OwnerID: ownerID, Name: name}
	q := tx.Where("name = ?", name)
	if groupID != nil {
		q = q.Where("group_id = ?", *groupID)
	} else {
		q = q.Where("owner_id = ?", *ownerID)
	}
	err := q.FirstOrCreate(&tag).Error
	return tag, err
}

// retag points a user's chores that carry tag from at tag to, keeping one link
// per chore.
func retag(tx *gorm.DB, userID, from, to uint) error {
	// A chore already carrying both would break the unique link, so drop the old
	// link there instead of moving it.
	both := tx.Model(&ChoreTag{}).Select("chore_id").Where("tag_id = ?", to)
	if err := tx.Where("tag_id = ? AND chore_id IN (?) AND chore_id IN (?)", from, userChoreIDs(tx, userID), both).Delete(&ChoreTag{}).Error; err != nil {
		return err
	}
	return tx.Model(&ChoreTag{}).
		Where("tag_id = ? AND chore_id IN (?)", from, userChoreIDs(tx, userID)).
		Update("tag_id", to).Error
}

// ForkTagsToOwner gives a user who is leaving a group their own copy of every
// group tag on their chores, and clears their "Tags to Show" choices for the
// group. The group's tags are left for the others. Call it when they leave.
func ForkTagsToOwner(tx *gorm.DB, userID, groupID uint) error {
	var used []Tag
	err := tx.Where("group_id = ? AND id IN (?)", groupID,
		tx.Model(&ChoreTag{}).Select("tag_id").Where("chore_id IN (?)", userChoreIDs(tx, userID))).
		Find(&used).Error
	if err != nil {
		return err
	}
	for _, t := range used {
		own, err := FindOrCreateTag(tx, nil, &userID, t.Name)
		if err != nil {
			return err
		}
		if err := retag(tx, userID, t.ID, own.ID); err != nil {
			return err
		}
	}
	return tx.Where("user_id = ? AND tag_id IN (?)", userID, groupTagIDs(tx, groupID)).Delete(&HiddenTag{}).Error
}

// MoveTagsToGroup folds a user's personal tags into a group they are joining.
// A personal tag whose name already exists in the group becomes that group tag.
// Call it when they join.
func MoveTagsToGroup(tx *gorm.DB, userID, groupID uint) error {
	var own []Tag
	if err := tx.Where("owner_id = ?", userID).Find(&own).Error; err != nil {
		return err
	}
	for _, t := range own {
		shared, err := FindOrCreateTag(tx, &groupID, nil, t.Name)
		if err != nil {
			return err
		}
		if err := retag(tx, userID, t.ID, shared.ID); err != nil {
			return err
		}
		if err := tx.Where("tag_id = ?", t.ID).Delete(&ChoreTag{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&Tag{}, t.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

// DeleteGroup removes a group with its members, pending invites and tags. Each
// member keeps their tags as personal ones first, as if they had left.
func DeleteGroup(db *gorm.DB, groupID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var members []uint
		if err := tx.Model(&GroupMember{}).Where("group_id = ?", groupID).Pluck("user_id", &members).Error; err != nil {
			return err
		}
		for _, id := range members {
			if err := ForkTagsToOwner(tx, id, groupID); err != nil {
				return err
			}
		}
		for _, model := range []any{&ChoreTag{}, &HiddenTag{}} {
			if err := tx.Where("tag_id IN (?)", groupTagIDs(tx, groupID)).Delete(model).Error; err != nil {
				return err
			}
		}
		for _, model := range []any{&Tag{}, &GroupMember{}, &GroupInvite{}} {
			if err := tx.Where("group_id = ?", groupID).Delete(model).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&Group{}, groupID).Error
	})
}
