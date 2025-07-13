NIXOS-CLI-OPTION-TUI(1)

# NAME

nixos option (interactive) - query available NixOS options interactively

# SYNOPSIS

*nixos option* [QUERY]

# DESCRIPTION

*nixos option*, by default, spawns interactive TUI search mode for NixOS
options, unless *-n* is specified.

Refer to *nixos-cli-option(1)* for non-interactive mode usage.

# WINDOW LAYOUT

A purple border means that a given window is active. If a window is active, then
its keybinds will work.

The main windows are the:

- Input/Result List Window
- Preview Window
- Help Window (this one)
- Option Value Window

## Help Window

Use the arrow keys or _h_, _j_, _k_, and _l_ to scroll around.

_<Esc>_ or _q_ will close this help window.

## Option Input Window

Type anything into the input box and all available options that match will be
filtered into a list. Scroll this list with the up or down cursor keys, and the
information for that option will show in the option preview window.

_<Tab>_ moves to the option preview window.

_<Enter>_ previews that option's current value, if it is able to be evaluated.
This will toggle the option value window.

## Option Preview Window

Use the cursor keys or _h_, _j_, _k_, and _l_ to scroll around.

The input box is not updated when this window is active.

_<Tab>_ will move back to the input window for searching.

_<Enter>_ will also evaluate the value, if possible. This will toggle the option
value window.

## Option Value Window

Use the cursor keys or _h_, _j_, _k_, and _l_ to scroll around.

_<Esc>_ or _q_ will close this window.

# SEE ALSO

*nixos-cli-option(1)*

# AUTHORS

Maintained by the *nixos-cli* team. See the main man page *nixos-cli(1)* for
details.
