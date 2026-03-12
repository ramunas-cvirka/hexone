on run argv
	if (count of argv) is less than 3 then error "usage: layout_dmg.scpt <volume-name> <volume-path> <app-name>"
	set volumeName to item 1 of argv
	set volumePath to item 2 of argv
	set appName to item 3 of argv
	set bgAlias to POSIX file (volumePath & "/.background/dmg-background.png") as alias

	tell application "Finder"
		tell disk volumeName
			open
			delay 0.5
			set current view of container window to icon view
			set toolbar visible of container window to false
			set statusbar visible of container window to false
			set bounds of container window to {120, 120, 760, 568}
			set opts to the icon view options of container window
			set arrangement of opts to not arranged
			set icon size of opts to 144
			set text size of opts to 16
			set background picture of opts to bgAlias
			set position of item appName of container window to {170, 198}
			set position of item "Applications" of container window to {470, 198}
			update without registering applications
			delay 1
			close
			open
			update without registering applications
			delay 1
		end tell
	end tell
end run
