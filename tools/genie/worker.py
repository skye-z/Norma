"""Genie-TTS 工作进程：由应用拉起，经标准输入输出通信，不监听任何网络端口。

请求（stdin，每行一个 JSON）：{"id": 1, "text": "...", "emotion": "smile"}
响应（stdout）：先一行 JSON {"id": 1, "ok": true, "sample_rate": 32000, "bytes": N}，随后 N 字节 PCM16LE；
失败时 {"id": 1, "ok": false, "error": "..."}。就绪时先输出一行 {"ready": true}。
库自己的打印会被重定向到 stderr，保证 stdout 只有协议数据。
"""
import argparse, json, os, sys, tempfile

from text_fix import normalize_for_genie

# 协议通道使用原始 stdout 的副本，随后把 fd 1 指向 stderr，防止库的 print 污染协议
_proto = os.fdopen(os.dup(1), "wb", buffering=0)
os.dup2(2, 1)
sys.stdout = sys.stderr


def send(obj, payload=b""):
    _proto.write((json.dumps(obj, ensure_ascii=False) + "\n").encode("utf-8"))
    if payload:
        _proto.write(payload)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--data-dir", required=True)
    ap.add_argument("--char-dir", required=True)
    ap.add_argument("--language", default="zh")
    ap.add_argument("--refs", required=True, help="JSON: {emotion: {audio, text}}，必须含 default")
    a = ap.parse_args()

    os.environ["GENIE_DATA_DIR"] = a.data_dir  # 必须在导入库之前设置
    import genie_tts as genie
    import soundfile as sf

    refs = json.loads(a.refs)
    name = "norma"
    genie.load_character(name, os.path.join(a.char_dir, "tts_models"), a.language)
    current = None

    def use_reference(emotion):
        nonlocal current
        key = emotion if emotion in refs else "default"
        if key == current:
            return
        r = refs[key]
        genie.set_reference_audio(name, r["audio"], r["text"])
        current = key

    def synth(text, emotion):
        use_reference(emotion)
        text = normalize_for_genie(text)
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "o.wav")
            genie.tts(name, text, play=False, split_sentence=False, save_path=path)
            data, rate = sf.read(path, dtype="int16")
        if data.ndim > 1:
            data = data[:, 0]
        return rate, data.tobytes()

    synth("你好。", "default")  # 预热，吃掉首次推理的初始化耗时
    send({"ready": True})

    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            req = json.loads(line)
            rate, pcm = synth(req["text"], req.get("emotion", "default"))
            send({"id": req["id"], "ok": True, "sample_rate": rate, "bytes": len(pcm)}, pcm)
        except Exception as e:  # 单次失败不应让进程退出
            rid = req.get("id") if isinstance(locals().get("req"), dict) else None
            send({"id": rid, "ok": False, "error": str(e)})


if __name__ == "__main__":
    main()
