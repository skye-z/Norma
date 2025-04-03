class NormaMicCaptureProcessor extends AudioWorkletProcessor {
  constructor() {
    super();
    this.targetSampleRate = 16000;
    this.flushSamples = 4096;
    this.buffer = [];
    this.port.onmessage = (event) => {
      const data = event && event.data ? event.data : {};
      if (Number.isFinite(data.targetSampleRate) && data.targetSampleRate > 0) {
        this.targetSampleRate = data.targetSampleRate;
      }
      if (Number.isFinite(data.flushSamples) && data.flushSamples > 0) {
        this.flushSamples = data.flushSamples;
      }
      if (data.flush === true) {
        this.emit(true);
      }
    };
  }

  downsample(input) {
    if (!input || input.length === 0) return new Float32Array(0);
    if (sampleRate === this.targetSampleRate) return input;
    const ratio = sampleRate / this.targetSampleRate;
    const length = Math.max(1, Math.round(input.length / ratio));
    const output = new Float32Array(length);
    for (let i = 0; i < length; i += 1) {
      const idx = Math.min(input.length - 1, Math.round(i * ratio));
      output[i] = input[idx];
    }
    return output;
  }

  emit(force = false) {
    if (!force && this.buffer.length < this.flushSamples) return;
    if (this.buffer.length === 0) return;
    const chunk = new Float32Array(this.buffer);
    this.buffer = [];
    this.port.postMessage({ type: 'frames', frames: chunk }, [chunk.buffer]);
  }

  process(inputs) {
    const input = inputs[0];
    if (!input || !input[0] || input[0].length === 0) return true;
    const downsampled = this.downsample(input[0]);
    for (let i = 0; i < downsampled.length; i += 1) {
      this.buffer.push(downsampled[i]);
    }
    this.emit(false);
    return true;
  }
}

registerProcessor('norma-mic-capture', NormaMicCaptureProcessor);
