package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type PhotoFrame struct {
	app        fyne.App
	window     fyne.Window
	imageView  *canvas.Image
	currentIdx int
	images     []string
}

func NewPhotoFrame() *PhotoFrame {
	a := app.New()
	a.Settings().SetTheme(&CustomTheme{})

	w := a.NewWindow("Domino Frame")

	return &PhotoFrame{
		app:        a,
		window:     w,
		currentIdx: 0,
		images:     []string{},
	}
}

func (pf *PhotoFrame) setupUI() {
	pf.imageView = canvas.NewImageFromResource(nil)
	pf.imageView.FillMode = canvas.ImageFillContain
	pf.imageView.ScaleMode = canvas.ImageScaleSmooth

	leftZone := widget.NewButton("", func() {
		pf.previousImage()
	})
	leftZone.Importance = widget.LowImportance

	rightZone := widget.NewButton("", func() {
		pf.nextImage()
	})
	rightZone.Importance = widget.LowImportance

	buttonLayout := container.NewAdaptiveGrid(2)
	buttonLayout.Add(leftZone)
	buttonLayout.Add(rightZone)
	layout := container.New(layout.NewStackLayout(), pf.imageView, buttonLayout)

	hiddenCursorWidget := NewHiddenCursorContainer(layout)

	pf.window.SetContent(hiddenCursorWidget)
	pf.window.SetFullScreen(true)
	pf.window.CenterOnScreen()

	go func() {
		time.Sleep(3 * time.Second)
		pf.window.Canvas().SetOnTypedKey(func(event *fyne.KeyEvent) {
			switch event.Name {
			case fyne.KeyLeft:
				pf.previousImage()
			case fyne.KeyRight:
				pf.nextImage()
			case fyne.KeyEscape:
				pf.app.Quit()
			}
		})
	}()
}

func (pf *PhotoFrame) loadImage(path string) {
	// pf.imageView = canvas.NewImageFromURI(s3URI)

	pf.imageView.File = path
	pf.imageView.FillMode = canvas.ImageFillContain
	pf.imageView.Refresh()
}

func (pf *PhotoFrame) nextImage() {
	if len(pf.images) == 0 {
		return
	}

	pf.currentIdx = (pf.currentIdx + 1) % len(pf.images)
	pf.loadImage(pf.images[pf.currentIdx])
	fmt.Println("Next image:", pf.images[pf.currentIdx])
}

func (pf *PhotoFrame) previousImage() {
	if len(pf.images) == 0 {
		return
	}

	pf.currentIdx = (pf.currentIdx - 1 + len(pf.images)) % len(pf.images)
	pf.loadImage(pf.images[pf.currentIdx])
	fmt.Println("Previous image:", pf.images[pf.currentIdx])
}

func (pf *PhotoFrame) loadImagesFromS3() {
	// TODO: Implement S3 sync logic here
	// For now, add some dummy images for testing

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	pf.images = []string{
		exPath + "/images/image1.jpg",
		exPath + "/images/image2.jpg",
		exPath + "/images/image3.jpg",
	}

	if len(pf.images) > 0 {
		pf.loadImage(pf.images[0])
	}
}

func (pf *PhotoFrame) startSlideshow(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			fyne.DoAndWait(func() {
				pf.nextImage()
			})
		}
	}()
}

func (pf *PhotoFrame) run() {
	pf.setupUI()
	pf.loadImagesFromS3()
	pf.startSlideshow(10 * time.Second)
	pf.window.ShowAndRun()
}

func main() {
	frame := NewPhotoFrame()
	frame.run()
}
