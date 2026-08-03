package main

import (
	"context"
	"fmt"
	"image/color"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
)

type PhotoFrame struct {
	app            fyne.App
	window         fyne.Window
	imageView      *canvas.Image
	imageMotion    *fyne.Animation
	imageLayout    *kenBurnsLayout
	motionSequence int
	currentIdx     int
	images         []string
	s3Client       *s3.Client
	bucketName     string
	imagesDir      string
	syncMutex      sync.RWMutex
	frameId        string
}

type kenBurnsLayout struct {
	image     *canvas.Image
	size      fyne.Size
	progress  float32
	direction fyne.Position
}

type tapZone struct {
	widget.BaseWidget
	onTapped func()
}

func newTapZone(onTapped func()) *tapZone {
	zone := &tapZone{onTapped: onTapped}
	zone.ExtendBaseWidget(zone)
	return zone
}

func (z *tapZone) Tapped(_ *fyne.PointEvent) {
	z.onTapped()
}

func (z *tapZone) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (l *kenBurnsLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	l.size = size
	l.updateImage()
}

func (l *kenBurnsLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(1, 1)
}

func (l *kenBurnsLayout) setProgress(progress float32) {
	l.progress = progress
	l.updateImage()
}

func (l *kenBurnsLayout) updateImage() {
	const zoom = 0.025

	scale := float32(1) + zoom*l.progress
	imageSize := fyne.NewSize(l.size.Width*scale, l.size.Height*scale)
	extraWidth := imageSize.Width - l.size.Width
	extraHeight := imageSize.Height - l.size.Height

	// Keep the image almost centered while it drifts into a different crop.
	x := -extraWidth/2 + l.direction.X*extraWidth/4
	y := -extraHeight/2 + l.direction.Y*extraHeight/4
	l.image.Move(fyne.NewPos(x, y))
	l.image.Resize(imageSize)
}

func NewPhotoFrame(server *SetupServer) *PhotoFrame {
	godotenv.Load()

	a := app.New()
	a.Settings().SetTheme(&CustomTheme{})

	w := a.NewWindow("Domino Frame")

	// Initialize S3 client for Cloudflare R2
	staticProvider := credentials.NewStaticCredentialsProvider(
		os.Getenv("R2_ACCESS_KEY"),
		os.Getenv("R2_SECRET_KEY"),
		"", // No session token needed for R2
	)
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithCredentialsProvider(staticProvider),
		config.WithRegion("auto"),
	)
	if err != nil {
		log.Printf("Unable to load SDK config: %v", err)
	}

	// Configure R2 endpoint
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", os.Getenv("CF_ACCOUNT_ID")))
	})

	// Get executable path for images directory
	ex, err := os.Executable()
	if err != nil {
		log.Printf("Error getting executable path: %v", err)
	}
	exPath := filepath.Dir(ex)
	imagesDir := filepath.Join(exPath, "images", server.frameId)

	// Create images directory if it doesn't exist
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		log.Printf("Image path didn't exist; Error creating images directory: %v", err)
	}

	return &PhotoFrame{
		app:        a,
		window:     w,
		currentIdx: 0,
		images:     []string{},
		s3Client:   s3Client,
		bucketName: os.Getenv("R2_BUCKET_NAME"),
		imagesDir:  imagesDir,
		frameId:    server.frameId,
	}
}

func (pf *PhotoFrame) setupUI() {
	pf.imageView = canvas.NewImageFromResource(nil)
	pf.imageView.FillMode = canvas.ImageFillContain
	pf.imageView.ScaleMode = canvas.ImageScaleSmooth
	pf.imageLayout = &kenBurnsLayout{image: pf.imageView}
	imageLayer := container.New(pf.imageLayout, pf.imageView)

	// Create black background rectangle
	blackBg := canvas.NewRectangle(color.Black)

	leftZone := newTapZone(func() {
		pf.previousImage()
	})

	rightZone := newTapZone(func() {
		pf.nextImage()
	})

	buttonLayout := container.NewAdaptiveGrid(2)
	buttonLayout.Add(leftZone)
	buttonLayout.Add(rightZone)
	layout := container.New(layout.NewStackLayout(), blackBg, imageLayer, buttonLayout)

	// hiddenCursorWidget := NewHiddenCursorContainer(layout)

	pf.window.SetContent(layout)
	pf.window.SetPadded(false)
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("Error getting hostname")
	}
	// Hacky way to only go fullscreen on non-development machines
	if hostname != "ndo-gb" {
		pf.window.SetFullScreen(true)
	}
	pf.window.CenterOnScreen()

	go func() {
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

	// image := canvas.NewImageFromFile(path)
	// pf.imageView.FillMode = canvas.ImageFillContain
	// pf.imageView.Image = image
	// pf.imageView.Refresh()
	// fmt.Println("loadImage.canvasLoaded")
	// pf.imageView.File = path
	// fmt.Println("loadImage.refreshed")
	// uri, err := storage.ParseURI(path)
	// if err != nil {
	// 	panic(fmt.Sprintf("Error parsing %s", c.photoUrl))
	// }
	// image := canvas.NewImageFromURI(uri)
	// image.FillMode = canvas.ImageFillContain

	pf.imageView.File = path

	fyne.Do(func() {
		pf.imageView.Refresh()
		pf.startImageMotion()
	})
}

func (pf *PhotoFrame) startImageMotion() {
	if pf.imageMotion != nil {
		pf.imageMotion.Stop()
	}

	directions := []fyne.Position{
		fyne.NewPos(-1, -1),
		fyne.NewPos(1, -1),
		fyne.NewPos(1, 1),
		fyne.NewPos(-1, 1),
	}
	pf.imageLayout.direction = directions[pf.motionSequence%len(directions)]
	pf.motionSequence++
	pf.imageLayout.setProgress(0)

	pf.imageMotion = fyne.NewAnimation(10*time.Second, pf.imageLayout.setProgress)
	pf.imageMotion.Curve = fyne.AnimationLinear
	pf.imageMotion.Start()
}

func (pf *PhotoFrame) nextImage() {
	pf.syncMutex.RLock()
	defer pf.syncMutex.RUnlock()

	if len(pf.images) == 0 {
		return
	}

	pf.currentIdx = (pf.currentIdx + 1) % len(pf.images)
	fmt.Println("Loading Next image:", pf.images[pf.currentIdx])
	pf.loadImage(pf.images[pf.currentIdx])
}

func (pf *PhotoFrame) previousImage() {
	pf.syncMutex.RLock()
	defer pf.syncMutex.RUnlock()

	if len(pf.images) == 0 {
		return
	}

	pf.currentIdx = (pf.currentIdx - 1 + len(pf.images)) % len(pf.images)
	fmt.Println("Loading previous image", pf.images[pf.currentIdx])
	pf.loadImage(pf.images[pf.currentIdx])
}

func (pf *PhotoFrame) syncS3Images() error {
	if pf.bucketName == "" {
		log.Println("S3_BUCKET_NAME not set, skipping S3 sync")
		return pf.loadLocalImages()
	}

	result, err := pf.s3Client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: &pf.bucketName,
		Prefix: &pf.frameId,
	})
	if err != nil {
		log.Printf("Failed to list S3 objects: %v", err)
		return pf.loadLocalImages()
	}

	// Filter for image files
	s3Images := make(map[string]bool)
	for _, obj := range result.Contents {
		key := *obj.Key

		// Remove frameId prefix (e.g. "dd3dk2a/filename.jpg" -> "filename.jpg")
		prefix := pf.frameId + "/"
		filename := strings.TrimPrefix(key, prefix)

		if pf.isImageFile(key) {
			s3Images[filename] = true

			// Download if not exists locally
			localPath := filepath.Join(pf.imagesDir, filename)
			if _, err := os.Stat(localPath); os.IsNotExist(err) {
				log.Printf("Downloading image: %s", localPath)
				if err := pf.downloadS3Object(key, localPath); err != nil {
					log.Printf("Failed to download %s: %v", key, err)
				}
			}
		}
	}

	// Remove local files that no longer exist in S3
	files, err := os.ReadDir(pf.imagesDir)
	if err != nil {
		log.Printf("Failed to read images directory: %v", err)
	} else {
		for _, file := range files {
			if !file.IsDir() && pf.isImageFile(file.Name()) {
				if !s3Images[file.Name()] {
					localPath := filepath.Join(pf.imagesDir, file.Name())
					log.Printf("Deleting file: %s", localPath)
					if err := os.Remove(localPath); err != nil {
						log.Printf("Failed to remove %s: %v", localPath, err)
					}
				}
			}
		}
	}

	return pf.loadLocalImages()
}

func (pf *PhotoFrame) isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif"
}

func (pf *PhotoFrame) downloadS3Object(key, localPath string) error {
	result, err := pf.s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: &pf.bucketName,
		Key:    &key,
	})
	if err != nil {
		return err
	}
	defer result.Body.Close()

	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Copy S3 object to local file
	_, err = io.Copy(file, result.Body)
	return err
}

func (pf *PhotoFrame) loadLocalImages() error {
	files, err := os.ReadDir(pf.imagesDir)
	if err != nil {
		return err
	}

	images := []string{}
	for _, file := range files {
		if !file.IsDir() && pf.isImageFile(file.Name()) {
			images = append(images, filepath.Join(pf.imagesDir, file.Name()))
		}
	}

	pf.syncMutex.Lock()
	pf.images = images
	pf.syncMutex.Unlock()

	if len(pf.images) > 0 {
		pf.loadImage(pf.images[0])
	}

	if len(pf.images) == 0 {
		log.Printf("No images available, exiting")
		os.Exit(0)
	}
	return nil
}

// func (pf *PhotoFrame) loadImagesFromS3() {
// 	if err := pf.syncS3Images(); err != nil {
// 		log.Printf("Failed to sync S3 images: %v", err)
// 	}
// }

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

func (pf *PhotoFrame) startS3Sync() {
	// Initial sync
	go func() {
		if err := pf.syncS3Images(); err != nil {
			log.Printf("Initial S3 sync failed: %v", err)
		}
	}()

	// Periodic sync every 10 minutes
	syncTicker := time.NewTicker(10 * time.Minute)
	go func() {
		for range syncTicker.C {
			log.Println("Starting periodic S3 sync...")
			if err := pf.syncS3Images(); err != nil {
				log.Printf("Periodic S3 sync failed: %v", err)
			} else {
				log.Println("S3 sync completed successfully")
			}
		}
	}()
}

func (pf *PhotoFrame) run() {
	pf.setupUI()
	pf.startS3Sync()
	pf.startSlideshow(10 * time.Second)
	pf.window.ShowAndRun()
}

func main() {
	server := NewSetupServer()

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start BLE server: %v", err)
	}
	log.Println("BLE server running. Press Ctrl+C to exit.")

	frame := NewPhotoFrame(server)
	frame.run()
}
