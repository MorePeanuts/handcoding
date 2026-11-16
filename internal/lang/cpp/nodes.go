package cpp

var nodeTypes = map[string]string{
	"GraphNode": `
#define HC_GRAPH
class Node {public: int val; vector<Node*> neighbors; Node():val(0){} Node(int v):val(v){} Node(int v,vector<Node*> n):val(v),neighbors(move(n)){} };
`,
	"NaryNode": `
#define HC_NARY
class Node {public: int val; vector<Node*> children; Node():val(0){} Node(int v):val(v){} Node(int v,vector<Node*> c):val(v),children(move(c)){} };
`,
	"RandomListNode": `
#define HC_RANDOM
class Node {public: int val; Node *next=nullptr,*random=nullptr; Node(int v):val(v){} };
`,
	"NextTreeNode": `
#define HC_NEXT
class Node {public: int val=0; Node *left=nullptr,*right=nullptr,*next=nullptr; Node()=default; Node(int v):val(v){} Node(int v,Node* l,Node* r,Node* n):val(v),left(l),right(r),next(n){} };
`,
	"QuadNode": `
#define HC_QUAD
class Node {public: bool val=false,isLeaf=false; Node *topLeft=nullptr,*topRight=nullptr,*bottomLeft=nullptr,*bottomRight=nullptr; Node()=default; Node(bool v,bool l):val(v),isLeaf(l){} Node(bool v,bool l,Node* tl,Node* tr,Node* bl,Node* br):val(v),isLeaf(l),topLeft(tl),topRight(tr),bottomLeft(bl),bottomRight(br){} };
`,
	"MultilevelNode": `
#define HC_MULTI
class Node {public: int val=0; Node *prev=nullptr,*next=nullptr,*child=nullptr; Node()=default; Node(int v):val(v){} Node(int v,Node* p,Node* n,Node* c):val(v),prev(p),next(n),child(c){} };
`,
}
