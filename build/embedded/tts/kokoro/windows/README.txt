This folder contains embedded sherpa-onnx Kokoro TTS assets for Windows builds.

Expected contents from kokoro-multi-lang-v1_1.tar.bz2:
- model.onnx
- voices.bin
- tokens.txt
- espeak-ng-data/
- lexicon-us-en.txt
- lexicon-zh.txt
- date-zh.fst
- number-zh.fst
- phone-zh.fst

The build scripts will copy these files into:
resource/runtime/providers/tts/current/
