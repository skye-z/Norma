package audio

import "math"

// EnergyVAD is a lightweight amplitude-based VAD.
type EnergyVAD struct {
	threshold float64
}

func NewEnergyVAD(threshold float64) *EnergyVAD {
	if threshold <= 0 {
		threshold = 0.02
	}
	return &EnergyVAD{threshold: threshold}
}

func (v *EnergyVAD) Detect(frame []float32) (bool, error) {
	if len(frame) == 0 {
		return false, nil
	}

	var sum float64
	for _, x := range frame {
		sum += math.Abs(float64(x))
	}
	avg := sum / float64(len(frame))
	return avg >= v.threshold, nil
}
