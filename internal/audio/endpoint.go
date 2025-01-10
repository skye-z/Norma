package audio

// EndpointDetector emits speech start/end signals.
type EndpointDetector struct {
    silenceFrames int
    inSpeech      bool
    silentCount   int
}

func NewEndpointDetector(silenceFrames int) *EndpointDetector {
    if silenceFrames <= 0 {
        silenceFrames = 4
    }
    return &EndpointDetector{silenceFrames: silenceFrames}
}

func (d *EndpointDetector) Update(isSpeech bool) (speechStart bool, speechEnd bool) {
    if isSpeech {
        if !d.inSpeech {
            d.inSpeech = true
            d.silentCount = 0
            return true, false
        }
        d.silentCount = 0
        return false, false
    }

    if !d.inSpeech {
        return false, false
    }

    d.silentCount++
    if d.silentCount >= d.silenceFrames {
        d.inSpeech = false
        d.silentCount = 0
        return false, true
    }
    return false, false
}
