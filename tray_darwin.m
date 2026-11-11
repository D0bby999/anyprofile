//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later

#import <Cocoa/Cocoa.h>
#include "_cgo_export.h"

#define ANYPROFILE_TRAY_OPEN     1
#define ANYPROFILE_TRAY_SETTINGS 2
#define ANYPROFILE_TRAY_UPDATES  3
#define ANYPROFILE_TRAY_STOP_ALL 4
#define ANYPROFILE_TRAY_QUIT     5

static NSImage *anyprofileCreateMenuBarIcon(BOOL active) {
	NSImage *img = [NSImage imageWithSize:NSMakeSize(18.0, 18.0)
	                              flipped:YES
	                       drawingHandler:^BOOL(NSRect dstRect) {
		(void)dstRect;
		CGContextRef cg = [[NSGraphicsContext currentContext] CGContext];
		[NSGraphicsContext saveGraphicsState];
		CGContextSetAlpha(cg, active ? 1.0 : 0.78);
		CGContextBeginTransparencyLayer(cg, NULL);

		NSAffineTransform *tx = [NSAffineTransform transform];
		[tx translateXBy:9.0 yBy:9.0];
		[tx scaleBy:(15.0 / 19.0)];
		[tx translateXBy:-12.0 yBy:-12.0];
		[tx concat];

		NSColor *ink = [NSColor blackColor];
		[ink setFill];
		[ink setStroke];

		// Back card peeking behind the primary profile card.
		NSBezierPath *backCard = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(6.5, 2.5, 15.0, 15.0)
		                                                         xRadius:3.8
		                                                         yRadius:3.8];
		[backCard setLineWidth:1.5];
		[backCard stroke];

		// Clear a crisp gap so the front card cleanly separates from the back card stroke.
		[[NSGraphicsContext currentContext] setCompositingOperation:NSCompositingOperationClear];
		NSBezierPath *halo = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(1.6, 5.6, 16.8, 16.8)
		                                                     xRadius:4.3
		                                                     yRadius:4.3];
		[halo fill];

		// Solid front card.
		[[NSGraphicsContext currentContext] setCompositingOperation:NSCompositingOperationSourceOver];
		NSBezierPath *frontCard = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(2.5, 6.5, 15.0, 15.0)
		                                                          xRadius:3.8
		                                                          yRadius:3.8];
		[frontCard fill];

		// Cut out user silhouette inside the front card.
		[[NSGraphicsContext currentContext] setCompositingOperation:NSCompositingOperationClear];
		NSBezierPath *head = [NSBezierPath bezierPathWithOvalInRect:NSMakeRect(7.7, 9.9, 4.6, 4.6)];
		[head fill];

		NSBezierPath *shoulders = [NSBezierPath bezierPath];
		[shoulders moveToPoint:NSMakePoint(5.9, 18.6)];
		[shoulders curveToPoint:NSMakePoint(14.1, 18.6)
		          controlPoint1:NSMakePoint(6.9, 15.4)
		          controlPoint2:NSMakePoint(13.1, 15.4)];
		[shoulders setLineWidth:1.8];
		[shoulders setLineCapStyle:NSLineCapStyleRound];
		[shoulders stroke];

		CGContextEndTransparencyLayer(cg);
		[NSGraphicsContext restoreGraphicsState];
		return YES;
	}];
	[img setTemplate:YES];
	return img;
}

@interface AnyProfileTrayController : NSObject
@property (nonatomic, strong) NSStatusItem *statusItem;
@property (nonatomic, strong) NSMenu *menu;
@property (nonatomic, strong) NSMenuItem *statusMenuItem;
@property (nonatomic, strong) NSMenuItem *openMenuItem;
@property (nonatomic, strong) NSMenuItem *settingsMenuItem;
@property (nonatomic, strong) NSMenuItem *updatesMenuItem;
@property (nonatomic, strong) NSMenuItem *stopAllMenuItem;
@property (nonatomic, strong) NSMenuItem *quitMenuItem;
@property (nonatomic, strong) NSImage *activeIcon;
@property (nonatomic, strong) NSImage *idleIcon;
@end

@implementation AnyProfileTrayController

- (instancetype)initWithActive:(BOOL)active
                    hasRunning:(BOOL)hasRunning
                   statusLabel:(NSString *)statusLabel
                     openLabel:(NSString *)openLabel
                 settingsLabel:(NSString *)settingsLabel
                  updatesLabel:(NSString *)updatesLabel
                  stopAllLabel:(NSString *)stopAllLabel
                     quitLabel:(NSString *)quitLabel
                       tooltip:(NSString *)tooltip {
	self = [super init];
	if (!self) {
		return nil;
	}

	_activeIcon = anyprofileCreateMenuBarIcon(YES);
	_idleIcon = anyprofileCreateMenuBarIcon(NO);

	_statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
	_menu = [[NSMenu alloc] initWithTitle:@"AnyProfile"];
	[_menu setAutoenablesItems:NO];

	_statusMenuItem = [[NSMenuItem alloc] initWithTitle:statusLabel action:nil keyEquivalent:@""];
	[_statusMenuItem setEnabled:NO];

	_openMenuItem = [[NSMenuItem alloc] initWithTitle:openLabel action:@selector(onOpen:) keyEquivalent:@""];
	[_openMenuItem setTarget:self];

	_settingsMenuItem = [[NSMenuItem alloc] initWithTitle:settingsLabel action:@selector(onSettings:) keyEquivalent:@""];
	[_settingsMenuItem setTarget:self];

	_updatesMenuItem = [[NSMenuItem alloc] initWithTitle:updatesLabel action:@selector(onUpdates:) keyEquivalent:@""];
	[_updatesMenuItem setTarget:self];

	_stopAllMenuItem = [[NSMenuItem alloc] initWithTitle:stopAllLabel action:@selector(onStopAll:) keyEquivalent:@""];
	[_stopAllMenuItem setTarget:self];

	_quitMenuItem = [[NSMenuItem alloc] initWithTitle:quitLabel action:@selector(onQuit:) keyEquivalent:@""];
	[_quitMenuItem setTarget:self];

	[_menu addItem:_statusMenuItem];
	[_menu addItem:[NSMenuItem separatorItem]];
	[_menu addItem:_openMenuItem];
	[_menu addItem:_settingsMenuItem];
	[_menu addItem:_updatesMenuItem];
	[_menu addItem:[NSMenuItem separatorItem]];
	[_menu addItem:_stopAllMenuItem];
	[_menu addItem:[NSMenuItem separatorItem]];
	[_menu addItem:_quitMenuItem];

	_statusItem.menu = _menu;
	[self applyActive:active
	       hasRunning:hasRunning
	      statusLabel:statusLabel
	        openLabel:openLabel
	    settingsLabel:settingsLabel
	     updatesLabel:updatesLabel
	     stopAllLabel:stopAllLabel
	        quitLabel:quitLabel
	          tooltip:tooltip];
	return self;
}

- (void)applyActive:(BOOL)active
         hasRunning:(BOOL)hasRunning
        statusLabel:(NSString *)statusLabel
          openLabel:(NSString *)openLabel
      settingsLabel:(NSString *)settingsLabel
       updatesLabel:(NSString *)updatesLabel
       stopAllLabel:(NSString *)stopAllLabel
          quitLabel:(NSString *)quitLabel
            tooltip:(NSString *)tooltip {
	if (_statusItem.button != nil) {
		_statusItem.button.image = active ? _activeIcon : _idleIcon;
		_statusItem.button.imagePosition = NSImageOnly;
		_statusItem.button.toolTip = tooltip;
	}
	[_statusMenuItem setTitle:statusLabel];
	[_openMenuItem setTitle:openLabel];
	[_settingsMenuItem setTitle:settingsLabel];
	[_updatesMenuItem setTitle:updatesLabel];
	[_stopAllMenuItem setTitle:stopAllLabel];
	[_stopAllMenuItem setEnabled:hasRunning];
	[_quitMenuItem setTitle:quitLabel];
}

- (void)onOpen:(id)sender {
	(void)sender;
	anyprofileTrayMenuAction(ANYPROFILE_TRAY_OPEN);
}

- (void)onSettings:(id)sender {
	(void)sender;
	anyprofileTrayMenuAction(ANYPROFILE_TRAY_SETTINGS);
}

- (void)onUpdates:(id)sender {
	(void)sender;
	anyprofileTrayMenuAction(ANYPROFILE_TRAY_UPDATES);
}

- (void)onStopAll:(id)sender {
	(void)sender;
	anyprofileTrayMenuAction(ANYPROFILE_TRAY_STOP_ALL);
}

- (void)onQuit:(id)sender {
	(void)sender;
	anyprofileTrayMenuAction(ANYPROFILE_TRAY_QUIT);
}

@end

static AnyProfileTrayController *gAnyProfileTray = nil;

static void anyprofileOnMainThread(dispatch_block_t block) {
	if ([NSThread isMainThread]) {
		block();
	} else {
		dispatch_async(dispatch_get_main_queue(), block);
	}
}

static NSString *anyprofileNSString(const char *s) {
	return s != NULL ? [NSString stringWithUTF8String:s] : @"";
}

void anyprofileTrayInit(
	int active,
	int hasRunning,
	const char *statusLabel,
	const char *openLabel,
	const char *settingsLabel,
	const char *updatesLabel,
	const char *stopAllLabel,
	const char *quitLabel,
	const char *tooltip) {
	BOOL isActive = active != 0;
	BOOL running = hasRunning != 0;
	NSString *status = anyprofileNSString(statusLabel);
	NSString *open = anyprofileNSString(openLabel);
	NSString *settings = anyprofileNSString(settingsLabel);
	NSString *updates = anyprofileNSString(updatesLabel);
	NSString *stopAll = anyprofileNSString(stopAllLabel);
	NSString *quit = anyprofileNSString(quitLabel);
	NSString *tip = anyprofileNSString(tooltip);

	anyprofileOnMainThread(^{
		if (gAnyProfileTray != nil) {
			[gAnyProfileTray applyActive:isActive
			                  hasRunning:running
			                 statusLabel:status
			                   openLabel:open
			               settingsLabel:settings
			                updatesLabel:updates
			                stopAllLabel:stopAll
			                   quitLabel:quit
			                     tooltip:tip];
			return;
		}
		gAnyProfileTray = [[AnyProfileTrayController alloc] initWithActive:isActive
		                                                        hasRunning:running
		                                                       statusLabel:status
		                                                         openLabel:open
		                                                     settingsLabel:settings
		                                                      updatesLabel:updates
		                                                      stopAllLabel:stopAll
		                                                         quitLabel:quit
		                                                           tooltip:tip];
	});
}

void anyprofileTrayUpdate(
	int active,
	int hasRunning,
	const char *statusLabel,
	const char *openLabel,
	const char *settingsLabel,
	const char *updatesLabel,
	const char *stopAllLabel,
	const char *quitLabel,
	const char *tooltip) {
	BOOL isActive = active != 0;
	BOOL running = hasRunning != 0;
	NSString *status = anyprofileNSString(statusLabel);
	NSString *open = anyprofileNSString(openLabel);
	NSString *settings = anyprofileNSString(settingsLabel);
	NSString *updates = anyprofileNSString(updatesLabel);
	NSString *stopAll = anyprofileNSString(stopAllLabel);
	NSString *quit = anyprofileNSString(quitLabel);
	NSString *tip = anyprofileNSString(tooltip);

	anyprofileOnMainThread(^{
		if (gAnyProfileTray == nil) {
			return;
		}
		[gAnyProfileTray applyActive:isActive
		                  hasRunning:running
		                 statusLabel:status
		                   openLabel:open
		               settingsLabel:settings
		                updatesLabel:updates
		                stopAllLabel:stopAll
		                   quitLabel:quit
		                     tooltip:tip];
	});
}

void anyprofileTrayDestroy(void) {
	anyprofileOnMainThread(^{
		if (gAnyProfileTray != nil) {
			[[NSStatusBar systemStatusBar] removeStatusItem:gAnyProfileTray.statusItem];
			gAnyProfileTray = nil;
		}
	});
}

void anyprofileTrayWakeRunLoop(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSEvent *ev = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
		                                 location:NSZeroPoint
		                            modifierFlags:0
		                                timestamp:0
		                             windowNumber:0
		                                  context:nil
		                                  subtype:0
		                                    data1:0
		                                    data2:0];
		[NSApp postEvent:ev atStart:YES];
	});
}

