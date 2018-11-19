// Code generated by reactGen. DO NOT EDIT.

package selectallbutton

import "myitcv.io/react"

type SelectAllBtnElem struct {
	react.Element
}

func buildSelectAllBtn(cd react.ComponentDef) react.Component {
	return SelectAllBtnDef{ComponentDef: cd}
}

func buildSelectAllBtnElem(props SelectAllBtnProps, children ...react.Element) *SelectAllBtnElem {
	return &SelectAllBtnElem{
		Element: react.CreateElement(buildSelectAllBtn, props, children...),
	}
}

func (s SelectAllBtnDef) RendersElement() react.Element {
	return s.Render()
}

// IsProps is an auto-generated definition so that SelectAllBtnProps implements
// the myitcv.io/react.Props interface.
func (s SelectAllBtnProps) IsProps() {}

// Props is an auto-generated proxy to the current props of SelectAllBtn
func (s SelectAllBtnDef) Props() SelectAllBtnProps {
	uprops := s.ComponentDef.Props()
	return uprops.(SelectAllBtnProps)
}

func (s SelectAllBtnProps) EqualsIntf(val react.Props) bool {
	return s == val.(SelectAllBtnProps)
}

var _ react.Props = SelectAllBtnProps{}
