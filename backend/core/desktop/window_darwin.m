#import <Cocoa/Cocoa.h>
#include "window_darwin.h"

static NSWindow *normaFind(const char *title) {
	NSString *want = [NSString stringWithUTF8String:title];
	for (NSWindow *w in [NSApp windows]) {
		if ([[w title] isEqualToString:want]) {
			return w;
		}
	}
	return nil;
}

static void onMainSync(void (^block)(void)) {
	if ([NSThread isMainThread]) {
		block();
	} else {
		dispatch_sync(dispatch_get_main_queue(), block);
	}
}

static void onMainAsync(void (^block)(void)) {
	if ([NSThread isMainThread]) {
		block();
	} else {
		dispatch_async(dispatch_get_main_queue(), block);
	}
}

int normaWindowExists(const char *title) {
	__block int ok = 0;
	onMainSync(^{ ok = normaFind(title) != nil; });
	return ok;
}

void normaSetIgnoreMouse(const char *title, int ignore) {
	NSString *t = [NSString stringWithUTF8String:title];
	onMainAsync(^{
		NSWindow *w = normaFind([t UTF8String]);
		if (w) [w setIgnoresMouseEvents:ignore ? YES : NO];
	});
}

void normaSetFloating(const char *title, int floating) {
	NSString *t = [NSString stringWithUTF8String:title];
	onMainAsync(^{
		NSWindow *w = normaFind([t UTF8String]);
		if (w) [w setLevel:floating ? NSFloatingWindowLevel : NSNormalWindowLevel];
	});
}

// 不在 Dock 与应用切换器里显示，与 Windows 隐藏任务栏图标一致
void normaHideFromDock(void) {
	onMainAsync(^{ [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory]; });
}

int normaCursorInteract(const char *title, int *inside, double *nx, double *ny) {
	__block int found = 0;
	__block NSRect frame = NSZeroRect;
	onMainSync(^{
		NSWindow *w = normaFind(title);
		if (w) {
			found = 1;
			frame = [w frame];
		}
	});
	if (!found || frame.size.width <= 0 || frame.size.height <= 0) return 0;

	NSPoint c = [NSEvent mouseLocation]; // 屏幕坐标，原点在左下
	double w = frame.size.width, h = frame.size.height;
	// 与 Windows 版相同：交互区近似可见的 Live2D 身体区域
	double left = frame.origin.x + w * 0.25;
	double right = frame.origin.x + w * 0.75;
	double topY = frame.origin.y + h * (1.0 - 0.08);    // 区域上沿（Cocoa 坐标 y 向上）
	double bottomY = frame.origin.y + h * (1.0 - 0.98); // 区域下沿
	double rw = right - left, rh = topY - bottomY;
	if (rw <= 0 || rh <= 0) return 0;

	*inside = (c.x >= left && c.x <= right && c.y >= bottomY && c.y <= topY) ? 1 : 0;
	double x = (((c.x - left) / rw) - 0.5) * 2;
	double y = ((((topY - c.y) / rh)) - 0.5) * 2; // 翻转为 y 向下为正
	if (x > 1) x = 1; if (x < -1) x = -1;
	if (y > 1) y = 1; if (y < -1) y = -1;
	*nx = x;
	*ny = y;
	return 1;
}
