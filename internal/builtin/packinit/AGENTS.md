# <% .Title %>

This repository is the hc pack `<% .Name %>`: problems in the format of
[handcoding (hc)](https://github.com/MorePeanuts/handcoding), which judges
them locally. A checkout of it is an hc authoring workspace: hc run anywhere
in it works on this pack, and `hc fetch`, `hc new` and `hc gen` add and
change problems in `problems/`.

- The problem protocol, the format of `pack.yaml` and of every problem, is
  defined by MorePeanuts/handcoding: `hc docs protocol` prints it, and
  `hc docs protocol <section>` one section of it.
- A problem lands only when `hc pack check` passes, which is what CI runs.
- If a problem cannot be expressed in the protocol, do not work around it:
  open an issue at https://github.com/MorePeanuts/handcoding/issues that says
  what is missing, and leave the problem out until the protocol has it.

<!-- Your own notes for agents go here. hc rewrites only the block between its markers below. -->
