package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	xdraw "golang.org/x/image/draw"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
)

type PhotoFrame struct {
	app    fyne.App
	window fyne.Window
	// Two stacked layers: imageBack holds the photo currently on screen, and
	// imageFront is the incoming one, faded in over the top.
	imageBack  *canvas.Image
	imageFront *canvas.Image
	imageFade  *fyne.Animation
	// Identifies the most recently requested photo, so a decode that finishes
	// after a newer one was asked for can be discarded.
	loadSeq         atomic.Uint64
	currentIdx      int
	images          []string
	s3Client        *s3.Client
	bucketName      string
	imagesDir       string
	syncMutex       sync.RWMutex
	frameId         string
	startedAt       time.Time
	reportHeartbeat func(string)
	activeImage     atomic.Value
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

// How long one photo takes to dissolve into the next.
//
// This animates Translucency rather than geometry, which is the whole point:
// position and size are rounded to whole pixels by the painter
// (roundToPixelCoords in internal/painter/gl/draw.go), so any slow movement
// advances in visible one-pixel steps. Alpha reaches the shader as a float
// uniform with no rounding at all, so a fade this slow is genuinely continuous
// where a pan of the same duration could never be.
const crossfadeDuration = 1500 * time.Millisecond

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
		startedAt:  time.Now(),
	}
}

func (pf *PhotoFrame) startHeartbeat(server *SetupServer) {
	report := func(activeImage string) {
		if activeImage == "" {
			if current, ok := pf.activeImage.Load().(string); ok {
				activeImage = current
			}
		}
		endpoint, hasEndpoint := server.getConfigValue("api_endpoint")
		apiKey, hasAPIKey := server.getConfigValue("api_key")
		apiEndpoint, endpointOK := endpoint.(string)
		key, keyOK := apiKey.(string)
		if !hasEndpoint || !hasAPIKey || !endpointOK || !keyOK || apiEndpoint == "" || key == "" {
			return
		}
		if !strings.HasPrefix(apiEndpoint, "https://") {
			log.Println("Metrics API endpoint must use HTTPS, skipping heartbeat")
			return
		}

		body, err := json.Marshal(map[string]any{
			"uptimeSeconds":         hostUptimeSeconds(),
			"storageTotalBytes":     storageTotalBytes(pf.imagesDir),
			"storageAvailableBytes": storageAvailableBytes(pf.imagesDir),
			"activeImage":           activeImage,
		})
		if err != nil {
			log.Printf("Failed to encode heartbeat: %v", err)
			return
		}
		request, err := http.NewRequest(
			http.MethodPost,
			strings.TrimRight(apiEndpoint, "/")+"/api/frames/heartbeat",
			bytes.NewReader(body),
		)
		if err != nil {
			log.Printf("Failed to create heartbeat request: %v", err)
			return
		}
		request.Header.Set("Authorization", "Bearer "+key)
		request.Header.Set("Content-Type", "application/json")

		response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
		if err != nil {
			log.Printf("Failed to report heartbeat: %v", err)
			return
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			log.Printf("Heartbeat failed with status: %s", response.Status)
			return
		}
		server.markMetricsActive()
	}

	pf.reportHeartbeat = report
	server.setConfigurationSavedCallback(func() { report("") })
	go report("")
	ticker := time.NewTicker(10 * time.Minute)
	go func() {
		for range ticker.C {
			report("")
		}
	}()
}

func hostUptimeSeconds() uint64 {
	contents, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(contents))
	if len(fields) == 0 {
		return 0
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return uint64(seconds)
}

func storageTotalBytes(path string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	return stat.Blocks * uint64(stat.Bsize)
}

func storageAvailableBytes(path string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	return stat.Bavail * uint64(stat.Bsize)
}

func (pf *PhotoFrame) setupUI() {
	// Fastest, not Smooth. Smooth makes Fyne run draw.CatmullRom.Scale on the
	// CPU every time the texture is invalidated (internal/painter/image.go),
	// which pinned the Pi to 2fps back when the animation invalidated it every
	// frame. Fastest returns the pixels untouched and lets the GPU scale; it
	// still filters with GL_LINEAR, so bilinear rather than nearest-neighbour.
	// decodeImage does one good CatmullRom pass per photo, so the GPU has
	// almost no scaling left to do.
	newLayer := func() *canvas.Image {
		layer := canvas.NewImageFromResource(nil)
		layer.FillMode = canvas.ImageFillContain
		layer.ScaleMode = canvas.ImageScaleFastest
		return layer
	}

	pf.imageBack = newLayer()
	pf.imageFront = newLayer()
	// Fully transparent until a crossfade runs.
	pf.imageFront.Translucency = 1

	imageLayer := container.NewStack(pf.imageBack, pf.imageFront)

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
	// Runs on Fyne's main goroutine -- the slideshow ticker wraps nextImage in
	// fyne.DoAndWait, and taps and key events are already on it -- so the canvas
	// can be read here, but nothing slow may happen here.
	panel := pf.window.Canvas().Size()

	// Decode off the render thread. Doing it inline froze the outgoing photo for
	// a few hundred milliseconds before every transition, since decode plus the
	// CatmullRom downscale is the most expensive thing per slide.
	//
	// The sequence number discards a decode that finished after a newer one was
	// requested: tapping through photos quickly starts several, and they do not
	// necessarily complete in order.
	seq := pf.loadSeq.Add(1)

	go func() {
		// Decoded rather than handed to Fyne as a path: canvas.Image.Refresh calls
		// updateReader, which does os.Open on File and decodes again, and the
		// crossfade refreshes every frame.
		decoded, err := decodeImage(path, panel)
		if err != nil {
			log.Printf("Failed to decode %s: %v", path, err)
			return
		}

		fyne.Do(func() {
			if pf.loadSeq.Load() != seq {
				return // superseded while decoding
			}

			// File must stay empty: updateReader checks it before Image and would
			// otherwise keep hitting the disk.
			pf.imageFront.File = ""
			pf.imageFront.Resource = nil
			pf.imageFront.Image = decoded
			pf.imageFront.Translucency = 1
			pf.imageFront.Refresh()

			pf.startCrossfade(filepath.Base(path))
		})
	}()
}

// startCrossfade dissolves imageFront in over imageBack, then promotes it.
func (pf *PhotoFrame) startCrossfade(activeImage string) {
	if pf.imageFade != nil {
		pf.imageFade.Stop()
	}

	incoming := pf.imageFront.Image

	pf.imageFade = fyne.NewAnimation(crossfadeDuration, func(progress float32) {
		pf.imageFront.Translucency = float64(1 - progress)
		// Translucency alone marks nothing dirty, so ask for a refresh.
		canvas.Refresh(pf.imageFront)

		if progress < 1 {
			return
		}

		// Promote the finished photo to the back layer and park the front one
		// transparent again, ready for the next arrival. The runner guarantees a
		// final Tick(1.0) (internal/animation/runner.go), so this always runs.
		pf.imageBack.File = ""
		pf.imageBack.Resource = nil
		pf.imageBack.Image = incoming
		pf.imageBack.Refresh()

		pf.imageFront.Translucency = 1
		canvas.Refresh(pf.imageFront)
		pf.activeImage.Store(activeImage)
		if pf.reportHeartbeat != nil {
			go pf.reportHeartbeat(activeImage)
		}
	})
	pf.imageFade.Curve = fyne.AnimationEaseInOut
	pf.imageFade.Start()
}

// decodeImage decodes a photo and downscales it once to fit the panel.
//
// Photos arrive up to 1920px wide from the web app's upload resize against a
// 1024x600 panel, so without this the GPU holds a texture with roughly three
// times more pixels than it can show -- and the crossfade blends both layers
// every frame, so there are two of them.
//
// Nothing is cropped: the whole photo stays visible, letterboxed by
// ImageFillContain where its aspect doesn't match the panel.
func decodeImage(path string, panel fyne.Size) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoded, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	// Before the window is laid out the panel size is unknown; keep full
	// resolution rather than guessing at a target.
	if panel.Width < 1 || panel.Height < 1 {
		return decoded, nil
	}

	source := decoded.Bounds()
	ratio := math.Min(
		float64(panel.Width)/float64(source.Dx()),
		float64(panel.Height)/float64(source.Dy()),
	)
	// Already small enough; upscaling here would only waste memory, since the
	// GPU can stretch it just as well.
	if ratio >= 1 {
		return decoded, nil
	}

	target := image.Rect(
		0, 0,
		int(math.Round(float64(source.Dx())*ratio)),
		int(math.Round(float64(source.Dy())*ratio)),
	)
	// CatmullRom is deliberate: it's the good kernel, and paying for it once per
	// photo is invisible against a 10s dwell. It is exactly what
	// ImageScaleSmooth was otherwise running on every single frame.
	out := image.NewRGBA(target)
	xdraw.CatmullRom.Scale(out, target, decoded, source, xdraw.Src, nil)
	return out, nil
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

func (pf *PhotoFrame) run(server *SetupServer) {
	pf.setupUI()
	pf.startS3Sync()
	pf.startHeartbeat(server)
	pf.startSlideshow(10 * time.Second)
	pf.window.ShowAndRun()
}

func main() {
	server := NewSetupServer()

	if err := server.Initialize(); err != nil {
		log.Fatalf("Failed to initialize frame configuration: %v", err)
	}
	if !server.IsConfigured() {
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start BLE server: %v", err)
		}
		log.Println("BLE setup server is running until metrics activation")
	} else {
		log.Println("Skipping BLE setup; metrics are already active")
	}

	frame := NewPhotoFrame(server)
	frame.run(server)
}
