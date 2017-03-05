package autorot

import (
	"image"
	"math"

	"github.com/unixpickle/anydiff"
	"github.com/unixpickle/anynet"
	"github.com/unixpickle/anyvec"
	"github.com/unixpickle/anyvec/anyvec32"
	"github.com/unixpickle/serializer"

	_ "github.com/unixpickle/anynet/anyconv"
)

// OutputType specifies the output format and loss
// function for a network.
type OutputType int

const (
	RawAngle OutputType = iota
	RightAngles
)

func init() {
	var n Net
	serializer.RegisterTypedDeserializer(n.SerializerType(), DeserializeNet)
}

// A Net is a neural net that predicts angles from images.
type Net struct {
	// Side length of input images.
	InputSize int

	OutputType OutputType
	Net        anynet.Net
}

// DeserializeNet deserializes a Net.
func DeserializeNet(d []byte) (*Net, error) {
	var res Net
	err := serializer.DeserializeAny(d, &res.InputSize, &res.OutputType, &res.Net)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// Evaluate generates a prediction for an image.
func (n *Net) Evaluate(img image.Image) float64 {
	if img.Bounds().Dx() != img.Bounds().Dy() ||
		img.Bounds().Dx() != n.InputSize {
		// Hack to crop the center square.
		img = Rotate(img, 0, n.InputSize)
	}
	inTensor := netInputTensor(img)
	inConst := anydiff.NewConst(anyvec32.MakeVectorData(inTensor))
	out := n.Net.Apply(inConst, 1).Output()
	switch n.OutputType {
	case RawAngle:
		return float64(anyvec.Sum(out).(float32))
	case RightAngles:
		return float64(anyvec.Sum(rightAngleMaxes(out)).(float32))
	default:
		panic("invalid OutputType")
	}
}

// Cost computes the total cost, given the desired output
// angles and the outputs from the network.
func (n *Net) Cost(desired, actual anydiff.Res, num int) anydiff.Res {
	if num != desired.Output().Len() {
		panic("bad batch size")
	}
	switch n.OutputType {
	case RawAngle:
		return anydiff.Complement(anydiff.Cos(anydiff.Sub(actual, desired)))
	case RightAngles:
		oneHots := anydiff.NewConst(rightAngleOneHots(desired.Output()))
		return anynet.DotCost{}.Cost(oneHots, actual, num)
	default:
		panic("invalid OutputType")
	}
}

// SerializerType returns the unique ID used to serialize
// a Net with the serializer package.
func (n *Net) SerializerType() string {
	return "github.com/unixpickle/autorot.Net"
}

// Serialize serializes the Net.
func (n *Net) Serialize() ([]byte, error) {
	return serializer.SerializeAny(
		serializer.Int(n.InputSize),
		serializer.Int(n.OutputType),
		n.Net,
	)
}

func rightAngleOneHots(angles anyvec.Vector) anyvec.Vector {
	// For each angle, we produce a one-hot vector indicating
	// which multiple of 90 degrees it is closest to.
	c := angles.Creator()
	stops := c.MakeNumericList([]float64{0, -math.Pi / 2, -math.Pi, -3 * math.Pi / 2})
	repeatedAngles := c.MakeVector(angles.Len() * 4)
	repeatedAngles.AddScaler(c.MakeNumeric(1))
	ones := repeatedAngles.Slice(0, angles.Len())
	anyvec.ScaleChunks(repeatedAngles, angles)
	anyvec.AddRepeated(repeatedAngles, c.MakeVectorData(stops))
	anyvec.Cos(repeatedAngles)
	maxMap := anyvec.MapMax(repeatedAngles, 4)
	repeatedAngles.Scale(c.MakeNumeric(0))
	maxMap.MapTranspose(ones, repeatedAngles)
	return repeatedAngles
}

func rightAngleMaxes(softOut anyvec.Vector) anyvec.Vector {
	c := softOut.Creator()
	stops := c.MakeNumericList([]float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2})
	repeatedAngles := c.MakeVector(softOut.Len())
	anyvec.AddRepeated(repeatedAngles, c.MakeVectorData(stops))
	maxes := anyvec.MapMax(softOut, 4)
	res := c.MakeVector(softOut.Len() / 4)
	maxes.Map(repeatedAngles, res)
	return res
}
