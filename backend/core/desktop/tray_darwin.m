#import <Cocoa/Cocoa.h>
#include "tray_darwin.h"
#include "_cgo_export.h"

@interface NormaTrayTarget : NSObject
- (void)onItem:(NSMenuItem *)sender;
@end

@implementation NormaTrayTarget
- (void)onItem:(NSMenuItem *)sender {
	normaTrayClicked((int)sender.tag);
}
@end

static NSStatusItem *normaStatusItem = nil;
static NSMenu *normaMenu = nil;
static NormaTrayTarget *normaTarget = nil;

static void onMain(void (^block)(void)) {
	if ([NSThread isMainThread]) {
		block();
	} else {
		dispatch_async(dispatch_get_main_queue(), block);
	}
}

void normaTrayInit(const char *tooltip) {
	NSString *tip = [NSString stringWithUTF8String:tooltip];
	onMain(^{
		normaTarget = [[NormaTrayTarget alloc] init];
		normaMenu = [[NSMenu alloc] initWithTitle:@"Norma"];
		[normaMenu setAutoenablesItems:NO];
		normaStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
		normaStatusItem.button.toolTip = tip;
		normaStatusItem.button.title = @"N"; // 没有图标时的占位，设置图标后会清空
		normaStatusItem.menu = normaMenu;
	});
}

void normaTraySetIcon(const void *png, int len) {
	NSData *data = [NSData dataWithBytes:png length:(NSUInteger)len];
	onMain(^{
		NSImage *img = [[NSImage alloc] initWithData:data];
		if (img == nil || normaStatusItem == nil) return;
		[img setSize:NSMakeSize(18, 18)];
		normaStatusItem.button.image = img;
		normaStatusItem.button.title = @"";
	});
}

void normaTrayAddItem(const char *title, int tag) {
	NSString *t = [NSString stringWithUTF8String:title];
	onMain(^{
		if (normaMenu == nil) return;
		NSMenuItem *it = [[NSMenuItem alloc] initWithTitle:t action:@selector(onItem:) keyEquivalent:@""];
		it.target = normaTarget;
		it.tag = tag;
		[normaMenu addItem:it];
	});
}

void normaTrayAddSeparator(void) {
	onMain(^{
		if (normaMenu != nil) [normaMenu addItem:[NSMenuItem separatorItem]];
	});
}

void normaTraySetTitle(int tag, const char *title) {
	NSString *t = [NSString stringWithUTF8String:title];
	onMain(^{
		NSMenuItem *it = [normaMenu itemWithTag:tag];
		if (it) it.title = t;
	});
}
