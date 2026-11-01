"""送入 Genie 前的文本规整。

Genie 的中文转拼音遇到“嗯”会抛 IndexError，整句合成静默失败（没有音频文件）。
“嗯”在对话里非常常见（应答语“嗯？”、回复开头的“嗯，……”），所以统一替换成发音相近、
能正常合成的“恩”。只收录已在 Genie 上实测会失败的字，不做其他改写。
"""

_REPLACEMENTS = {"嗯": "恩"}
_TABLE = str.maketrans(_REPLACEMENTS)


def normalize_for_genie(text: str) -> str:
    return text.translate(_TABLE)
