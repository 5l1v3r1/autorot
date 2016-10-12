package autorot

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/unixpickle/resize"
	"github.com/unixpickle/sgd"
	"github.com/unixpickle/weakai/neuralnet"
)

// A Sample stores a training sample for a network.
type Sample struct {
	// Path is the path to the unrotated image.
	Path string

	// Angle is the angle at which the image should be
	// rotated before being fed to the network.
	Angle float64
}

// A SampleSet is an sgd.SampleSet that generates
// neuralnet.VectorSample instances based on the pixels of
// rotated image samples.
type SampleSet struct {
	Samples   []Sample
	ImageSize int
}

// ReadSampleSet walks the directory and creates a sample
// for each of the images (with a random rotation).
func ReadSampleSet(imageSize int, dir string) (*SampleSet, error) {
	res := &SampleSet{ImageSize: imageSize}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		ext := filepath.Ext(path)
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			sample := Sample{Path: path, Angle: randomAngle()}
			res.Samples = append(res.Samples, sample)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Len returns the number of samples in the set.
func (s *SampleSet) Len() int {
	return len(s.Samples)
}

// Swap swaps two sample indices.
func (s *SampleSet) Swap(i, j int) {
	s.Samples[i], s.Samples[j] = s.Samples[j], s.Samples[i]
}

// GetSample generates a rotated and scaled image tensor
// for the given sample index.
// It returns a neuralnet.VectorSample.
func (s *SampleSet) GetSample(idx int) interface{} {
	sample := s.Samples[idx]
	f, err := os.Open(sample.Path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		panic(err)
	}
	rotated := Rotate(img, sample.Angle)
	return neuralnet.VectorSample{
		Input:  netInputTensor(rotated, s.ImageSize).Data,
		Output: []float64{sample.Angle},
	}
}

// Copy creates a copy of the sample set.
func (s *SampleSet) Copy() sgd.SampleSet {
	res := &SampleSet{
		ImageSize: s.ImageSize,
		Samples:   make([]Sample, len(s.Samples)),
	}
	copy(res.Samples, s.Samples)
	return res
}

// Subset returns a subset of the sample set.
func (s *SampleSet) Subset(i, j int) sgd.SampleSet {
	return &SampleSet{
		ImageSize: s.ImageSize,
		Samples:   s.Samples[i:j],
	}
}

func randomAngle() float64 {
	return rand.Float64() * math.Pi * 2
}

func netInputTensor(img image.Image, size int) *neuralnet.Tensor3 {
	scaled := resize.Resize(uint(size), uint(size), img, resize.Bilinear)
	res := neuralnet.NewTensor3(size, size, 3)
	for x := 0; x < size; x++ {
		for y := 0; y < size; y++ {
			pixel := scaled.At(x+scaled.Bounds().Min.X,
				y+scaled.Bounds().Min.Y)
			r, g, b, _ := pixel.RGBA()
			res.Set(x, y, 0, float64(r)/0xffff)
			res.Set(x, y, 1, float64(g)/0xffff)
			res.Set(x, y, 2, float64(b)/0xffff)
		}
	}
	return res
}
