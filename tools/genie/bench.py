"""Genie-TTS 对比测试：加载、首次推理、逐句流式首块时间、实时率，并保存试听文件。
用法：GENIE_DATA_DIR=<GenieData> python bench.py <角色模型目录> <参考音频> <参考文本> <输出目录>
"""
import asyncio, json, os, sys, time, statistics

char_dir, ref_wav, ref_text, out_dir = sys.argv[1:5]
import genie_tts as genie
import soundfile as sf

SENTENCES = [
    "你好呀，今天过得怎么样？",
    "好，我讲一个很短的：有个人去图书馆借书，管理员问他借什么。",
    "记住了，你养的猫叫团子，以后聊到它，我会记得名字。",
    "听起来不错，说说是什么让今天变得顺利吧。",
    "最近睡得不太好的话，可以试试睡前半小时不看手机。",
    "明天下午三点有个会，我到时候提醒你。",
]
os.makedirs(out_dir, exist_ok=True)

t0 = time.time()
genie.load_character("feibi", os.path.join(char_dir, "tts_models"), "zh")
genie.set_reference_audio("feibi", ref_wav, ref_text)
load_s = time.time() - t0
print(f"加载 {load_s:.2f}s")

async def stream_once(text):
    t = time.time(); first = None; nbytes = 0
    async for chunk in genie.tts_async("feibi", text, play=False, split_sentence=False):
        if first is None:
            first = time.time() - t
        nbytes += len(chunk)
    return first, time.time() - t, nbytes

def save_once(text, path):
    t = time.time()
    genie.tts("feibi", text, play=False, split_sentence=False, save_path=path)
    return time.time() - t

# 首次推理（含各会话初始化）
first_total = save_once(SENTENCES[0], os.path.join(out_dir, "warmup.wav"))
print(f"首次推理 {first_total*1000:.0f} ms")

rows = []
for r in range(2):
    for i, s in enumerate(SENTENCES):
        path = os.path.join(out_dir, f"s{i}.wav")
        gen = save_once(s, path)
        info = sf.info(path)
        dur = info.frames / info.samplerate
        first, total, nb = asyncio.run(stream_once(s))
        rows.append(dict(text=s, gen_ms=gen * 1000, audio_ms=dur * 1000, rtf=gen / dur,
                         stream_first_chunk_ms=(first or 0) * 1000, stream_total_ms=total * 1000, sample_rate=info.samplerate))
g = sum(x["gen_ms"] for x in rows); a = sum(x["audio_ms"] for x in rows)
print(f"整句合成：平均 {g/len(rows):.0f} ms，平均音频 {a/len(rows):.0f} ms，实时率 {g/a:.3f}")
print(f"流式首块：平均 {statistics.mean(x['stream_first_chunk_ms'] for x in rows):.0f} ms，"
      f"中位 {statistics.median(x['stream_first_chunk_ms'] for x in rows):.0f} ms")
json.dump(dict(load_s=load_s, first_call_ms=first_total * 1000, rtf=g / a, rows=rows), open(os.path.join(out_dir, "result.json"), "w"), ensure_ascii=False, indent=1)
