#import <Carbon/Carbon.h>
#include "hotkeys_darwin.h"
#include "_cgo_export.h"

static EventHotKeyRef normaRefs[4];
static EventHandlerRef normaHandler = NULL;

static OSStatus normaOnHotkey(EventHandlerCallRef next, EventRef ev, void *ud) {
	EventHotKeyID hk;
	if (GetEventParameter(ev, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(hk), NULL, &hk) == noErr) {
		normaHotkeyPressed((int)hk.id);
	}
	return noErr;
}

void normaRegisterHotkeys(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (normaHandler == NULL) {
			EventTypeSpec spec = {kEventClassKeyboard, kEventHotKeyPressed};
			InstallApplicationEventHandler(&normaOnHotkey, 1, &spec, NULL, &normaHandler);
		}
		// 与 Windows 版一致：Option+T 置顶，Option+P 穿透，Option+N 通知模式，Option+H 提示
		const UInt32 keys[4] = {kVK_ANSI_T, kVK_ANSI_P, kVK_ANSI_N, kVK_ANSI_H};
		for (int i = 0; i < 4; i++) {
			EventHotKeyID id = {'norm', (UInt32)(i + 1)};
			RegisterEventHotKey(keys[i], optionKey, id, GetApplicationEventTarget(), 0, &normaRefs[i]);
		}
	});
}

void normaUnregisterHotkeys(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		for (int i = 0; i < 4; i++) {
			if (normaRefs[i]) {
				UnregisterEventHotKey(normaRefs[i]);
				normaRefs[i] = NULL;
			}
		}
	});
}
