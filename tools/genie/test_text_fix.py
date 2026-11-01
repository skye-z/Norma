import unittest

from text_fix import normalize_for_genie


class NormalizeForGenieTest(unittest.TestCase):
    def test_replaces_char_that_crashes_g2p(self):
        self.assertEqual(normalize_for_genie("嗯？"), "恩？")
        self.assertEqual(normalize_for_genie("嗯，我在听。"), "恩，我在听。")
        self.assertEqual(normalize_for_genie("嗯嗯，好的"), "恩恩，好的")

    def test_leaves_other_text_untouched(self):
        for s in ["你好，今天过得怎么样？", "{{smile}}在的", "Hello 123", ""]:
            self.assertEqual(normalize_for_genie(s), s)


if __name__ == "__main__":
    unittest.main()
