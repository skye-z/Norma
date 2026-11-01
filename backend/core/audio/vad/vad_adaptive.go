package audio

import (
	"math"
	"sync"
)

// AdaptiveVAD 是自带底噪估计的能量 VAD。
// 固定阈值只适合某一种麦克风电平：实测笔记本麦克风在关闭自动增益时，说话的帧平均幅度只有 0.005~0.015，
// 低于 0.02 的固定阈值，整句话都会被当成静音。这里持续估计环境底噪，阈值取底噪的数倍并设绝对下限，
// 无论麦克风电平高低都能分出“在说话”和“安静”。
type AdaptiveVAD struct {
	// 阈值的绝对下限，避免在极安静的环境里被数字噪声触发
	MinThreshold float64
	// 阈值 = 底噪 × Ratio
	Ratio float64
	// 连续判为“说话”超过这么多帧，认为是环境变响了而不是一直在说话，重新估计底噪
	MaxSpeechRun int

	mu        sync.Mutex
	noise     float64
	ready     bool
	speechRun int
}

func NewAdaptiveVAD() *AdaptiveVAD {
	return &AdaptiveVAD{MinThreshold: 0.004, Ratio: 3.5, MaxSpeechRun: 600}
}

func meanAbs(frame []float32) float64 {
	var sum float64
	for _, x := range frame {
		sum += math.Abs(float64(x))
	}
	return sum / float64(len(frame))
}

// 当前阈值，供诊断使用
func (v *AdaptiveVAD) Threshold() float64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.thresholdLocked()
}

func (v *AdaptiveVAD) thresholdLocked() float64 {
	t := v.noise * v.Ratio
	if t < v.MinThreshold {
		t = v.MinThreshold
	}
	return t
}

func (v *AdaptiveVAD) Detect(frame []float32) (bool, error) {
	if len(frame) == 0 {
		return false, nil
	}
	avg := meanAbs(frame)

	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.ready {
		v.noise = avg
		v.ready = true
	}
	speech := avg >= v.thresholdLocked()
	if speech {
		v.speechRun++
		if v.MaxSpeechRun > 0 && v.speechRun > v.MaxSpeechRun {
			// 持续很久都在“说话”，更可能是环境底噪抬高了：向当前电平靠拢，重新开始判断
			v.noise = avg * 0.8
			v.speechRun = 0
		}
		return true, nil
	}
	v.speechRun = 0
	// 只用安静帧更新底噪：变小时跟得快，变大时跟得慢，避免把一句话的起头吃进底噪
	if avg < v.noise {
		v.noise = v.noise*0.9 + avg*0.1
	} else {
		v.noise = v.noise*0.98 + avg*0.02
	}
	return false, nil
}
