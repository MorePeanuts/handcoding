struct Node* intersect(struct Node* a, struct Node* b) {
    if (a->isLeaf) return a->val ? a : b;
    if (b->isLeaf) return b->val ? b : a;
    struct Node *tl = intersect(a->topLeft, b->topLeft),
                *tr = intersect(a->topRight, b->topRight);
    struct Node *bl = intersect(a->bottomLeft, b->bottomLeft),
                *br = intersect(a->bottomRight, b->bottomRight);
    struct Node* out = calloc(1, sizeof(*out));
    if (tl->isLeaf && tr->isLeaf && bl->isLeaf && br->isLeaf &&
        tl->val == tr->val && tl->val == bl->val && tl->val == br->val) {
        out->isLeaf = true;
        out->val = tl->val;
    } else {
        out->val = true;
        out->topLeft = tl;
        out->topRight = tr;
        out->bottomLeft = bl;
        out->bottomRight = br;
    }
    return out;
}
