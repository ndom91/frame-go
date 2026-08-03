package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
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
	xdraw "golang.org/x/image/draw"

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

// Temporary instrumentation for diagnosing jerky motion. Fyne invokes
// setProgress once per animation frame, so counting calls measures the
// effective frame rate directly, and timing updateImage attributes the cost to
// the per-frame image rescale. Remove once the motion is tuned.
var (
	motionFrames   int
	motionSpent    time.Duration
	motionWindowAt = time.Now()
)

func (l *kenBurnsLayout) setProgress(progress float32) {
	l.progress = progress

	// Animation callbacks all run on Fyne's main goroutine, so these counters
	// need no synchronisation.
	start := time.Now()
	l.updateImage()
	motionSpent += time.Since(start)
	motionFrames++

	if elapsed := time.Since(motionWindowAt); elapsed >= time.Second {
		log.Printf(
			"ken burns: %.1f fps, %.2f ms/frame in updateImage",
			float64(motionFrames)/elapsed.Seconds(),
			motionSpent.Seconds()*1000/float64(motionFrames),
		)
		motionFrames, motionSpent, motionWindowAt = 0, 0, time.Now()
	}
}

// How much larger than the panel each photo is rendered. The surplus is what
// the pan travels across, so it sets both the crop tightness and the speed.
//
// 0.20 on a 1024px panel gives 205px of travel over a 10s slide, about
// 20px/second. That matters because Fyne rounds geometry to whole pixels
// (roundToPixelCoords in internal/painter/gl/draw.go), so there is no sub-pixel
// interpolation to hide behind: the motion is visible as ~20 one-pixel steps a
// second, which reads as a drift. The previous 2.5% zoom moved 2.5px/second --
// about two steps a second -- which read as a twitch no matter the frame rate.
const kenBurnsOverscan = 0.20

func (l *kenBurnsLayout) updateImage() {
	if l.size.Width < 1 || l.size.Height < 1 {
		return
	}

	extraWidth := l.size.Width * kenBurnsOverscan
	extraHeight := l.size.Height * kenBurnsOverscan

	// Deliberately a pan, with no zoom. Resize invalidates the texture and makes
	// Fyne regenerate and re-upload it; Move only calls repaint and leaves the
	// cached texture alone. Since the size is constant for a given panel, this
	// call hits Resize's early return after the first frame and costs nothing.
	l.image.Resize(fyne.NewSize(l.size.Width+extraWidth, l.size.Height+extraHeight))

	// Travel the full surplus, corner to opposite corner, so progress 0 and 1
	// sit at the two extremes rather than drifting around the centre.
	x := -extraWidth/2 + l.direction.X*extraWidth*(l.progress-0.5)
	y := -extraHeight/2 + l.direction.Y*extraHeight*(l.progress-0.5)
	l.image.Move(fyne.NewPos(x, y))
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
	// Fastest, not Smooth. Smooth makes Fyne run draw.CatmullRom.Scale on the
	// CPU every time the texture is invalidated (internal/painter/image.go),
	// and the ken burns animation invalidates it on every frame -- which is
	// what pinned the Pi to 2 fps. Fastest returns the pixels untouched and
	// lets the GPU scale; it still filters with GL_LINEAR, so it is bilinear
	// rather than nearest-neighbour. decodeImage does one good CatmullRom pass
	// per slide so the GPU is only ever scaling up by the zoom fraction.
	pf.imageView.ScaleMode = canvas.ImageScaleFastest
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

	// Temporary, alongside the fps counter. Fyne rounds geometry in scaled units
	// (roundToPixelCoords), so a canvas scale other than 1.0 makes the smallest
	// possible movement larger than one device pixel -- which would cap how
	// smooth the pan can ever be, independently of frame rate or distance.
	go func() {
		time.Sleep(2 * time.Second)
		canvas := pf.window.Canvas()
		log.Printf(
			"canvas: scale=%.3f size=%.0fx%.0f (pixel quantum = %.2f device px)",
			canvas.Scale(), canvas.Size().Width, canvas.Size().Height,
			1/canvas.Scale(),
		)
	}()

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

	// Decode once here rather than handing Fyne the path.
	//
	// canvas.Image.Resize branches on `isSVG || Image == nil`. With only File
	// set, Image is nil, so every resize took the "rasterise at the new size"
	// path -- which calls Refresh, which re-opens the file and decodes the JPEG
	// again. The ken burns animation resizes on every frame, so the Pi was
	// decoding a full JPEG off disk twice a second to shift the image ~1.3px.
	// With Image populated, Resize instead takes the branch Fyne annotates as
	// "just re-size using GPU scaling".
	decoded, err := decodeImage(path, pf.imageLayout.size)
	if err != nil {
		log.Printf("Failed to decode %s: %v", path, err)
		return
	}

	fyne.Do(func() {
		// File must be cleared: updateReader prefers it over Image and would
		// still hit the disk on every Refresh.
		pf.imageView.File = ""
		pf.imageView.Resource = nil
		pf.imageView.Image = decoded

		pf.imageView.Refresh()
		pf.startImageMotion()
	})
}

// decodeImage decodes a photo and, if it is larger than the panel needs,
// downscales it once to just above panel size.
//
// This is the expensive resample, paid once per slide instead of once per frame.
// Photos arrive up to 1920px wide from the web app's upload resize, and the
// panel is 1024x600, so without this the GPU holds a texture with ~3x more
// pixels than it can show.
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

	// Before the first Layout the panel size is unknown; keep full resolution
	// rather than guessing at a target.
	if panel.Width < 1 || panel.Height < 1 {
		return decoded, nil
	}

	box := image.Rect(
		0, 0,
		int(math.Round(float64(panel.Width)*(1+kenBurnsOverscan))),
		int(math.Round(float64(panel.Height)*(1+kenBurnsOverscan))),
	)

	// Crop to the panel's aspect rather than fitting inside it. ImageFillContain
	// letterboxes anything that doesn't match, and panning a letterboxed image
	// just slides the black bars around instead of revealing more photo.
	source := decoded.Bounds()
	boxAspect := float64(box.Dx()) / float64(box.Dy())
	sourceAspect := float64(source.Dx()) / float64(source.Dy())

	crop := source
	switch {
	case sourceAspect > boxAspect: // too wide, trim the sides
		width := int(math.Round(float64(source.Dy()) * boxAspect))
		inset := (source.Dx() - width) / 2
		crop = image.Rect(source.Min.X+inset, source.Min.Y, source.Min.X+inset+width, source.Max.Y)
	case sourceAspect < boxAspect: // too tall, trim top and bottom
		height := int(math.Round(float64(source.Dx()) / boxAspect))
		inset := (source.Dy() - height) / 2
		crop = image.Rect(source.Min.X, source.Min.Y+inset, source.Max.X, source.Min.Y+inset+height)
	}

	// CatmullRom is deliberate: it's the good kernel, and paying for it once per
	// slide is invisible against a 10s dwell. It is exactly what
	// ImageScaleSmooth was otherwise running on every single frame.
	out := image.NewRGBA(box)
	xdraw.CatmullRom.Scale(out, box, decoded, crop, xdraw.Src, nil)
	return out, nil
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
