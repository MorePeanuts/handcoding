---
name: hc-problem
description: Write or fix the definition of an hc (handcoding) problem, what it is rather than how it is judged, that is its directory and identity, its problem.yaml signature (kind, parameters, structure inputs such as hidden, cycle, join, ref, copy and returns_ref, returns_copy, LeetCode's Node types, round_trip, in-place judge.output), its description.md and its templates. Use when writing a new problem in an hc workspace (`hc new`), checking a problem fetched with `hc fetch`, or fixing a problem whose signature, statement or templates are wrong. For the judge rule (judge.compare), reference solutions, checkers and test cases, use hc-cases instead.
---

# handcoding problem authoring

`hc` is a local judge for LeetCode-style problems. A workspace contains
`hc.yaml`; its own problems live in `packs/local/problems/<dir>/`, and
`hc new` and `hc fetch` put new ones there. Other directories of `packs/`
are installed packs: read-only, so never edit them (hc refuses to). The
user's own code is in `solutions/<pack>/<lang>/<dir>/`. **Never read or
modify the user's solutions unless asked to diagnose a reported failure,
and never change them.**

In hc commands, name a problem by its slug, or as `<pack>/<slug>` when the
slug is in several packs (`local/two-sum`); `hc gen` and `hc fix` tell you
the name to use.

This skill covers what a problem is: its directory, its signature, its
statement and its templates. How it is judged (`judge.compare`, reference
solutions, `checker.py`, `cases.jsonl`) is the `hc-cases` skill; a problem
written with this skill is finished with that one.

## The spec

The facts (every field, type encoding and rule) are in the problem protocol
that hc embeds. This skill does not repeat them: when a step names a
section, read it with `hc docs protocol <section>` before you write that
part, and follow it exactly. `hc docs protocol --list` lists the sections.
When this skill and the spec seem to disagree, the spec wins.

The sections you will need most:

- `hc docs protocol problem-directory`: the files of a problem
- `hc docs protocol identity`: slug, id and the directory name
- `hc docs protocol problemyaml`: every field of problem.yaml
- `hc docs protocol types`: type names and their JSON encodings
- `hc docs protocol signatures`: function and class signatures
- `hc docs protocol validity-rules`: what `hc validate` enforces

## Identity: never rename

A problem's slug is its identity, and the directory name is derived from
its id and slug. The user's solutions and submission history are keyed by
the directory name. **Never rename a problem directory, and never change
the `id` or `slug` of an existing problem**, even when they look wrong. If
the user asks for it, explain that it detaches their solutions and history,
and let them do it.

For a new problem (`hc docs protocol identity`):

- A LeetCode problem keeps LeetCode's slug and number as `slug` and `id`
  (`two-sum`, `"1"`).
- A problem of your own gets a kebab-case slug that names it, and either no
  id or a number no other problem uses. Check with `hc list --plain`.

## Decisions

Each decision below reads the statement (and LeetCode's template, when the
problem has one) and ends in fields of problem.yaml. Make them in this
order.

### Kind

Read `hc docs protocol function-problems` and
`hc docs protocol class-problems`.

- `function`: the solution is one function (LeetCode's `class Solution`
  with one method).
- `class`: a design problem. The statement asks to implement a class, and
  its examples are two arrays, the operations and their arguments
  (`["LRUCache","put","get"]`, `[[2],[1,1],[1]]`). Iterator problems (173
  Binary Search Tree Iterator, 341 Flatten Nested List Iterator) are class
  problems too, even when the statement shows a loop instead of arrays.
- An encoder and a decoder of a format the solution chooses is a class
  problem with a round trip; see [Round trips](#round-trips).

### Signature

- Copy names from LeetCode's template when there is one: the function,
  class and method names, and the parameter names in their order. Starters
  are generated from the signature, and a template must call the same
  names.
- Use LeetCode's type for each parameter and return value. For a problem of
  your own, take the type the statement implies: `long` when a value or
  result can exceed 32 bits, `double` for real numbers, `character` for a
  single character, arrays for lists. `hc docs protocol types` lists the
  names; quote array types inside `{...}` (`type: "integer[]"`).
- A function that returns nothing and modifies a parameter is in place; see
  [In-place output](#in-place-output).

### Structure inputs

Some function problems hand the solution linked structures a plain JSON
value cannot describe, or inputs the solution never sees. Read
`hc docs protocol structure-inputs` first. The statement tells you which
attribute applies:

| The statement says | Write |
|---|---|
| "`pos` is used to denote the index of the node that tail's next pointer is connected to", "`pos` is not passed as a parameter" (141, 142) | a hidden integer `pos`, and `cycle: pos` on the list parameter |
| "the judge creates the linked structure based on these inputs", `skipA` and `skipB` (160) | hidden `skipA`, `skipB`, and `join` on the second list; drop `intersectVal`, the driver does not need it |
| "`p` and `q` will exist in the tree", "`target` is a node of the tree" (235, 236, 863) | `ref: root` on each node parameter |
| "you are given the node to be deleted, you will not be given access to the head" (237) | a hidden list `head`, `ref: head` on `node`, and `judge.output: head` |
| "a reference to the same node in the cloned tree", where the solution also receives the clone (1379) | `copy: original` on `cloned`, `ref: original` on `target` |
| "return the node where the cycle begins", "return the node at which the lists intersect", "return the lowest common ancestor", "return a reference to the node" | `returns_ref` naming the structure the node belongs to |
| "return a deep copy", "clone the graph" (133, 138) | `returns_copy` naming the copied parameter; read `hc docs protocol returns_copy` |

- Keep the names the statement uses for hidden inputs (`pos`, `skipA`,
  `skipB`).
- Use `returns_ref` only when the answer is *which node*. A function that
  returns the head of a rearranged list (206 Reverse Linked List) is judged
  by the values after it, and needs nothing.
- A plain `ListNode` or `TreeNode` parameter needs no attribute.
- Design problems cannot use structure inputs.

### LeetCode's Node types

LeetCode calls six different structures `Node`; each has its own type name
in problem.yaml, but solution code still calls it `Node`. Pick the one
whose fields match the struct in the comment of LeetCode's Go template (or
the class in the Python one):

- `Neighbors`: `GraphNode`
- `Children`: `NaryNode`
- `Next` and `Random`: `RandomListNode`
- `Left`, `Right` and `Next`: `NextTreeNode`
- `IsLeaf` and the four quadrants (`TopLeft`, ...): `QuadNode`
- `Prev`, `Next` and `Child`: `MultilevelNode`

A problem uses at most one of them, never in arrays. Read
`hc docs protocol leetcodes-node-types` for their encodings, and
`hc docs protocol nestedinteger-and-employee` for the two other types
LeetCode's judge defines, which keep their names.

### Round trips

When a design problem asks for an encoder and a decoder in a format the
solution chooses, and only checks that decoding the encoded value gives it
back (297 Serialize and Deserialize Binary Tree, 449 Serialize and
Deserialize BST, 535 Encode and Decode TinyURL), set `judge.round_trip`.
Read `hc docs protocol round-trip-problems`.

- `encode` and `decode` are the two method names from the template.
- `instances` comes from the harness in the template's comment:
  `separate` when two objects are used
  (`ser = Codec(); deser = Codec(); ans = deser.deserialize(ser.serialize(root))`:
  297, 449), `same` when one object does both
  (`codec.decode(codec.encode(url))`: 535). Without a comment, use
  `separate` unless the decoder cannot work without the encoder's state
  (a URL shortener's mapping).
- Requirements on the encoded string itself ("as compact as possible") are
  not judged. Say so in the statement's hc note (below).

### In-place output

When the function returns nothing and the statement says to modify a
parameter ("do not return anything, modify `nums` in-place"), set
`judge.output` to that parameter. When it returns a length k and "it does
not matter what you leave beyond the first k elements" (26, 27, 80), also
set `output_length: return`. Read `hc docs protocol in-place-output`.

## description.md

The statement users read with `hc show <slug>`, in Markdown.

- Start with `# <id>. <title>` (`# <title>` without an id), then the
  statement, the examples and the constraints, as LeetCode shows them.
- Images go in the problem's `assets/` directory and are linked by relative
  path, `![](assets/example1.png)`. Never link remote images: hc works
  offline.
- **Write an hc note wherever hc's format differs from the statement's**,
  so the user knows what their output is compared with. Put it after the
  statement and before the examples, in the statement's language
  (`**hc note:**`, `**hc 说明：**`), and give the examples' inputs or
  outputs in hc's form. Typical differences:
  - `NextTreeNode` outputs as its levels read through the next pointers,
    `[[1],[2,3],[4,5,7]]`, not LeetCode's `[1,#,2,3,#,4,5,7,#]` (116, 117);
  - a `returns_ref` output is the node's index in a list or its value in a
    tree, not `tail connects to node index 1` or `Intersected at '8'`
    (142, 160);
  - inputs that hc writes differently, such as 160's full lists with
    `skipA` and `skipB` and no `intersectVal`;
  - an iterator problem's output is one value per call, `null` for the
    constructor (341);
  - a requirement hc does not judge (449's compactness).
- `hc docs protocol cases` describes each form the note may need.

## Templates

`templates/solution.go` and `templates/solution.py` are optional: without
them hc generates the starter from the signature. When the problem is on
LeetCode, use LeetCode's templates.

- A template must keep the signature exactly (names, parameter order,
  types), because the driver calls it.
- LeetCode's templates can be copied as they are: hc adds `package main` to
  a Go template that lacks it, and its import header to a Python one.
- Do not declare types hc predefines (`ListNode`, `TreeNode`, `Node`,
  `NestedInteger`, `Employee`); keep LeetCode's comment that shows them.
  Read `hc docs protocol solution-abi` for what each language provides.

## Workflow: write a new problem

1. If the problem is on LeetCode, prefer `hc fetch <slug>` (it needs
   network): it imports the statement, images, templates, signature and
   examples. Then follow [check a fetched problem](#workflow-check-a-fetched-problem).
   Otherwise go on.
2. Scaffold it: `hc new <slug> --id <id> --title "<title>"`, adding
   `--kind class` for a design problem (leave out `--id` for a problem
   without one). Never create the directory by hand.
3. Make the [decisions](#decisions) and write `problem.yaml`. Leave
   `judge.compare` unset; deciding it is part of `hc-cases`.
4. Write `description.md`, with the images in `assets/`.
5. Write templates if the problem has some.
6. Add the statement's examples to `cases.jsonl` as `"example": true`
   cases, with the `expected` output the statement gives, in hc's encoding
   (`hc docs protocol cases`). They let `hc validate --run` check the
   reference against the statement later. When the statement accepts
   several answers, leave `expected` out: how they are judged is decided
   in `hc-cases`.
7. Run `hc validate <slug>`. Fix every error; the warnings about the
   undecided judge rule, missing expected outputs and the missing reference
   are for `hc-cases`.
8. Continue with the `hc-cases` skill: the judge rule, the reference
   solutions and the test cases.

## Workflow: check a fetched problem

`hc fetch` converts LeetCode's data and prints warnings for what it could
not. Read them if you ran it. Then check:

1. **The signature matches the templates.** Compare `function` (or
   `class`) in problem.yaml with `templates/solution.go` and
   `templates/solution.py`: names, order and types of the parameters. For
   LeetCode's judge-defined types hc reads the signature from the Go
   template; check that the `Node` type matches the struct's fields
   ([LeetCode's Node types](#leetcodes-node-types)).
2. **Structure inputs.** hc fetch derives `hidden`, `cycle`, `join`, `ref`,
   `copy` and `returns_ref` from LeetCode's test data. Keep them, and check
   them against [the statement](#structure-inputs). It never sets
   `returns_copy`: add it when the function returns a deep copy.
3. **Round trip and in-place output.** Keep `judge.round_trip`, and check
   `instances` against the template's comment. Keep `judge.output` and
   `output_length`.
4. **Examples.** Open `cases.jsonl`:
   - fill the example outputs it left empty, in hc's encoding: outputs the
     statement writes in another form, such as a `returns_ref` output
     (142's `tail connects to node index 1` is `1`; 160's
     `Intersected at '8'` is the node's index in `headA`, `2`);
   - iterator problems (341) lose their examples, because LeetCode gives
     only the constructor's input: write them as `"example": true` class
     cases whose `ops` follow the statement's loop (`hasNext`, then `next`,
     until `hasNext` is false).
5. **description.md.** Add an [hc note](#descriptionmd) wherever hc's
   format differs, and check that every image is in `assets/`
   (`hc validate <slug>` warns about missing ones).
6. Run `hc validate <slug>`, and continue with `hc-cases`: a fetched
   problem has no judge rule yet, and if hc fetch warned that LeetCode
   judges it with its own checker, expect several valid answers.

Never run `hc fetch --force` on a problem that exists: it replaces the
directory and keeps nothing, cases and references included.

## Workflow: fix a problem's definition

You were told what is wrong, or found it while working on the cases
(`hc-cases` sends you here when the signature, a structure input, the
statement or a template is wrong).

1. Read `problem.yaml`, `description.md` and `templates/` against the
   statement, and the spec section for the part in question.
2. Fix the definition. Never change `id` or `slug`, and never rename the
   directory.
3. If the inputs changed (a parameter renamed, a hidden input added, a
   `ref` instead of a whole structure), update every case in `cases.jsonl`
   to the new inputs, and the reference solutions to the new signature.
   Expected outputs computed under the old definition are suspect: continue
   with `hc-cases`, which recomputes them with `hc cases build <slug> --force`
   and runs `hc validate <slug> --run`.
4. Otherwise run `hc validate <slug>` until it reports no errors.

## Summary

Report what you wrote or changed and why: the kind and signature, each
structure input or Node type with the sentence of the statement it rests
on, and the hc notes you added. If the signature changed, tell the user
that solutions they already wrote must follow the new one; do not touch
them.
