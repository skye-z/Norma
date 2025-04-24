class NormaMicCaptureProcessor extends AudioWorkletProcessor {
  constructor() {
    super();
    this.flushSamples = 4096;
    this.buffer = [];
    this.port.onmessage = (event) => {
      const data = event && event.data ? event.data : {};
      if (Number.isFinite(data.flushSamples) && data.flushSamples > 0) {
        this.flushSamples = data.flushSamples;
      }
      if (data.flush === true) {
        this.emit(true);
      }
    };
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
    const channel = input[0];
    for (let i = 0; i < channel.length; i += 1) {
      this.buffer.push(channel[i]);
    }
    this.emit(false);
    return true;
  }
}

registerProcessor('norma-mic-capture', NormaMicCaptureProcessor);
