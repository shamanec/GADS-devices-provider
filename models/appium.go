package models

type ActionData struct {
	X          float64 `json:"x,omitempty"`
	Y          float64 `json:"y,omitempty"`
	EndX       float64 `json:"endX,omitempty"`
	EndY       float64 `json:"endY,omitempty"`
	TextToType string  `json:"text,omitempty"`
}

type DeviceAction struct {
	Type     string  `json:"type"`
	Duration int     `json:"duration"`
	X        float64 `json:"x,omitempty"`
	Y        float64 `json:"y,omitempty"`
	Button   int     `json:"button"`
	Origin   string  `json:"origin,omitempty"`
}

type DeviceActionParameters struct {
	PointerType string `json:"pointerType"`
}

type DevicePointerAction struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Parameters DeviceActionParameters `json:"parameters"`
	Actions    []DeviceAction         `json:"actions"`
}

type DevicePointerActions struct {
	Actions []DevicePointerAction `json:"actions"`
}
