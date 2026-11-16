package c

var nodeTypes = map[string]string{
	"GraphNode":      "#define HC_GRAPH\nstruct Node {int val,numNeighbors; struct Node **neighbors;};\n",
	"NaryNode":       "#define HC_NARY\nstruct Node {int val,numChildren; struct Node **children;};\n",
	"RandomListNode": "#define HC_RANDOM\nstruct Node {int val; struct Node *next,*random;};\n",
	"NextTreeNode":   "#define HC_NEXT\nstruct Node {int val; struct Node *left,*right,*next;};\n",
	"QuadNode":       "#define HC_QUAD\nstruct Node {bool val,isLeaf; struct Node *topLeft,*topRight,*bottomLeft,*bottomRight;};\n",
	"MultilevelNode": "#define HC_MULTI\nstruct Node {int val; struct Node *prev,*next,*child;};\n",
}
