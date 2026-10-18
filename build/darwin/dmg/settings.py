# SPDX-License-Identifier: AGPL-3.0-or-later
# dmgbuild settings for the macOS disk image: the app beside an Applications
# link over a background whose arrow says to drag one onto the other. Run by
# scripts/make-dmg.sh, which passes the app as -D app=<path>.
import os.path

app = defines["app"]  # noqa: F821 (dmgbuild supplies defines)
here = defines["dir"]  # noqa: F821 (this folder, for the background)
name = os.path.basename(app)

format = "UDZO"
files = [app]
symlinks = {"Applications": "/Applications"}
icon = os.path.join(app, "Contents", "Resources", "iconfile.icns")
hide_extension = [name]

# The window is the background's size; the icons sit on its arrow.
background = os.path.join(here, "background.png")
window_rect = ((200, 140), (640, 400))
default_view = "icon-view"
show_status_bar = False
show_tab_view = False
show_toolbar = False
show_pathbar = False
show_sidebar = False
icon_size = 128
text_size = 13
arrange_by = None
icon_locations = {name: (170, 190), "Applications": (470, 190)}
