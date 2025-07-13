# FAQ

## Do you intend on making this an official NixOS application?

Yes! I will be writing an RFC to see if people are interested in this.

That's my ultimate goal, anyway.

## Aren't there other people doing what you're doing?

Yes! Check the [comparisons](./comparisons.md) page for a listing of there tools
as well as some pros/cons to each one.

## What about `home-manager` and `nix-darwin`? Will you support those systems?

They are fundamentally separate projects with roughly similar surfaces, so no. I
am a heavy user of both projects, though, so I may write my own `darwin` and
`hm` CLIs that roughly mirror this.

Think about this:

- `home-manager` has to work in the user context, while NixOS works in the
  system one.
- `nix-darwin` doesn't interact with boot scripts, while NixOS does.

Among a slew of other differences. The `rebuild` + `switch` workflow may be the
same, but the options are different, and I'm lazy. So no.

## Can the option search work with other sources?

Yes! However, this has been moved to a separate tool called
[`optnix`](https://github.com/water-sucks/optnix).

`nixos option` actually uses `optnix` as a library within its code, so it is
exactly the same UX.

## More questions?

File an issue! Perhaps it's important enough to add to this FAQ as well.
