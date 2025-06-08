package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type HiddenCursorContainer struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

func NewHiddenCursorContainer(content fyne.CanvasObject) *HiddenCursorContainer {
	h := &HiddenCursorContainer{content: content}
	h.ExtendBaseWidget(h)
	return h
}

func (h *HiddenCursorContainer) Cursor() desktop.Cursor {
	return desktop.HiddenCursor
}

func (h *HiddenCursorContainer) CreateRenderer() fyne.WidgetRenderer {
	return &hiddenCursorRenderer{content: h.content}
}

type hiddenCursorRenderer struct {
	content fyne.CanvasObject
}

func (r *hiddenCursorRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)
}

func (r *hiddenCursorRenderer) MinSize() fyne.Size {
	return r.content.MinSize()
}

func (r *hiddenCursorRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}

func (r *hiddenCursorRenderer) Refresh() {}
func (r *hiddenCursorRenderer) Destroy() {}
