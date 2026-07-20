// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>

char *hexoneMovePathToTrash(const char *path) {
	@autoreleasepool {
		NSString *value = [NSString stringWithUTF8String:path];
		if (value == nil) {
			return strdup("invalid UTF-8 path");
		}
		NSURL *url = [NSURL fileURLWithPath:value];
		NSError *error = nil;
		BOOL moved = [[NSFileManager defaultManager]
			trashItemAtURL:url
			resultingItemURL:NULL
			error:&error];
		if (moved) {
			return NULL;
		}
		NSString *description = error.localizedDescription;
		if (description == nil) {
			description = @"unable to move item to Trash";
		}
		return strdup(description.UTF8String);
	}
}
