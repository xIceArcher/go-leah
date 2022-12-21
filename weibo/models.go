package weibo

import (
	"encoding/json"
	"reflect"
)

type RawGetPostResp struct {
	ID       string                 `json:"idstr"`
	PicIDs   []string               `json:"pic_ids"`
	PicInfos map[string]RawPhotoMap `json:"pic_infos"`
}

type RawPhotoMap map[string]*RawPhoto

func (p *RawPhotoMap) UnmarshalJSON(bytes []byte) error {
	temp := make(map[string]interface{})
	if err := json.Unmarshal(bytes, &temp); err != nil {
		return err
	}

	*p = make(RawPhotoMap)
	for key, val := range temp {
		if reflect.ValueOf(val).Kind() == reflect.Map {
			valBytes, err := json.Marshal(val)
			if err != nil {
				return err
			}

			photo := &RawPhoto{}
			if err := json.Unmarshal(valBytes, photo); err != nil {
				continue
			}

			(*p)[key] = photo
		}
	}

	return nil
}

type RawPhoto struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Post struct {
	ID     string
	Photos []*Photo
}

type Photo struct {
	ID       string
	Variants []*PhotoVariant
}

func (p *Photo) BestVariant() *PhotoVariant {
	var bestVariant *PhotoVariant
	for _, variant := range p.Variants {
		if bestVariant == nil || variant.Resolution() > bestVariant.Resolution() {
			bestVariant = variant
		}
	}
	return bestVariant
}

type PhotoVariant struct {
	VariantName string
	URL         string
	Height      int
	Width       int
}

func (v *PhotoVariant) Resolution() int {
	return v.Height * v.Width
}
