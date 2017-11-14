// Code generated by reactGen. DO NOT EDIT.

package react

// HrProps defines the properties for the <hr> element
type HrProps struct {
	ClassName               string
	DangerouslySetInnerHTML *DangerousInnerHTML
	ID                      string
	Key                     string

	OnChange
	OnClick

	Role  string
	Style *CSS
}

func (h *HrProps) assign(v *_HrProps) {

	v.ClassName = h.ClassName

	v.DangerouslySetInnerHTML = h.DangerouslySetInnerHTML

	if h.ID != "" {
		v.ID = h.ID
	}

	if h.Key != "" {
		v.Key = h.Key
	}

	if h.OnChange != nil {
		v.o.Set("onChange", h.OnChange.OnChange)
	}

	if h.OnClick != nil {
		v.o.Set("onClick", h.OnClick.OnClick)
	}

	v.Role = h.Role

	// TODO: until we have a resolution on
	// https://github.com/gopherjs/gopherjs/issues/236
	v.Style = h.Style.hack()

}
