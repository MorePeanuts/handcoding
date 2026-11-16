package java

var nodeTypes = map[string]string{
	"GraphNode":      `class Node { public int val; public List<Node> neighbors; public Node(){this(0);} public Node(int v){this(v,new ArrayList<>());} public Node(int v,ArrayList<Node> n){this(v,(List<Node>)n);} public Node(int v,List<Node> n){val=v;neighbors=n;} }`,
	"NaryNode":       `class Node { public int val; public List<Node> children; public Node(){this(0);} public Node(int v){this(v,new ArrayList<>());} public Node(int v,List<Node> c){val=v;children=c;} }`,
	"RandomListNode": `class Node { public int val; public Node next,random; public Node(int v){val=v;} }`,
	"NextTreeNode":   `class Node { public int val; public Node left,right,next; public Node(){} public Node(int v){val=v;} public Node(int v,Node l,Node r,Node n){val=v;left=l;right=r;next=n;} }`,
	"QuadNode":       `class Node { public boolean val,isLeaf; public Node topLeft,topRight,bottomLeft,bottomRight; public Node(){} public Node(boolean v,boolean l){val=v;isLeaf=l;} public Node(boolean v,boolean l,Node tl,Node tr,Node bl,Node br){this(v,l);topLeft=tl;topRight=tr;bottomLeft=bl;bottomRight=br;} }`,
	"MultilevelNode": `class Node { public int val; public Node prev,next,child; public Node(){} public Node(int v){val=v;} }`,
}
