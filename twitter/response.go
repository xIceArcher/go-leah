package twitter

import "time"

type getSpaceResponse struct {
	Data *struct {
		ID    string `json:"id"`
		State string `json:"state"`

		StartedAt time.Time `json:"started_at"`
		EndedAt   time.Time `json:"ended_at"`

		Title            string `json:"title"`
		CreatorID        string `json:"creator_id"`
		ParticipantCount int    `json:"participant_count"`
	} `json:"data"`
	Includes *struct {
		Users []struct {
			ProfileImageURL string `json:"profile_image_url"`
			ID              string `json:"id"`
			Username        string `json:"username"`
			Name            string `json:"name"`
		} `json:"users"`
	} `json:"includes"`
	Errors []*struct {
		Value        string `json:"value"`
		Detail       string `json:"detail"`
		Title        string `json:"title"`
		ResourceType string `json:"resource_type"`
		Parameter    string `json:"parameter"`
		ResourceID   string `json:"resource_id"`
		Type         string `json:"type"`
	} `json:"errors"`
}

func (r *getSpaceResponse) toDTO() *Space {
	if len(r.Errors) > 0 || r.Data == nil {
		return &Space{}
	}

	space := &Space{
		ID:    r.Data.ID,
		Title: r.Data.Title,
		State: SpaceState(r.Data.State),

		ParticipantCount: r.Data.ParticipantCount,

		StartTime: r.Data.StartedAt,
		EndTime:   r.Data.EndedAt,
	}

	if r.Includes != nil {
		for _, includedUser := range r.Includes.Users {
			if includedUser.ID == r.Data.CreatorID {
				space.Creator = &User{
					ID:              includedUser.ID,
					Name:            includedUser.Name,
					ScreenName:      includedUser.Username,
					ProfileImageURL: includedUser.ProfileImageURL,
				}
			}
		}
	}

	return space
}
