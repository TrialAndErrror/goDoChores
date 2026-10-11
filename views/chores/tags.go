package chores

import (
	"errors"
	"goDoChores/database"
	"goDoChores/models"
	"goDoChores/views/groups"
	"net/url"
	"strconv"

	"gorm.io/gorm"
)

// TagPicker is what the chore form needs to render its tag dropdown.
type TagPicker struct {
	Tags     []models.Tag
	Selected map[uint]bool
}

func newTagPicker(userID uint, selected []uint) (TagPicker, error) {
	tags, err := groups.UserTags(database.DB, userID)
	if err != nil {
		return TagPicker{}, err
	}
	sel := make(map[uint]bool, len(selected))
	for _, id := range selected {
		sel[id] = true
	}
	return TagPicker{Tags: tags, Selected: sel}, nil
}

// tagIDsFromForm reads the chosen tag ids and checks that every one is one of the
// user's own tags (their group's, or personal ones when they have no group).
func tagIDsFromForm(userID uint, data url.Values) ([]uint, error) {
	raw := data["tag_ids"]
	if len(raw) == 0 {
		return nil, nil
	}
	allowed, err := groups.UserTags(database.DB, userID)
	if err != nil {
		return nil, err
	}
	valid := make(map[uint]bool, len(allowed))
	for _, t := range allowed {
		valid[t.ID] = true
	}
	seen := map[uint]bool{}
	var ids []uint
	for _, s := range raw {
		id, err := strconv.ParseUint(s, 10, 32)
		if err != nil || !valid[uint(id)] {
			return nil, errors.New("unknown tag")
		}
		if !seen[uint(id)] {
			seen[uint(id)] = true
			ids = append(ids, uint(id))
		}
	}
	if len(ids) > models.MaxTagsPerChore {
		return nil, errors.New("too many tags")
	}
	return ids, nil
}

// saveTags makes the chore carry exactly the given tags.
func saveTags(tx *gorm.DB, choreID uint, tagIDs []uint) error {
	if err := tx.Where("chore_id = ?", choreID).Delete(&models.ChoreTag{}).Error; err != nil {
		return err
	}
	for _, id := range tagIDs {
		if err := tx.Create(&models.ChoreTag{ChoreID: choreID, TagID: id}).Error; err != nil {
			return err
		}
	}
	return nil
}
