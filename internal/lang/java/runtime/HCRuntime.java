import java.io.*;
import java.lang.reflect.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.util.*;

final class HCRuntime {
  static final int MAX_NODES = 1_000_000;
  @SuppressWarnings("unchecked")
  static Map<String, Object> map(Object x) {
    return (Map<String, Object>) x;
  }
  @SuppressWarnings("unchecked")
  static List<Object> list(Object x) {
    return x == null ? new ArrayList<>() : (List<Object>) x;
  }
  static String str(Map<String, Object> m, String k) {
    Object x = m.get(k);
    return x == null ? "" : x.toString();
  }
  static int integer(Object x) {
    long n = ((Number) x).longValue();
    check(n >= Integer.MIN_VALUE && n <= Integer.MAX_VALUE, "integer outside signed 32-bit range");
    return (int) n;
  }
  static void check(boolean ok, String message) {
    if (!ok)
      throw new IllegalArgumentException(message);
  }
  static Set<Object> identities() {
    return Collections.newSetFromMap(new IdentityHashMap<>());
  }
  static String nodeVariant;
  static Object field(Object n, String name) throws Exception {
    Field f = n.getClass().getDeclaredField(name);
    f.setAccessible(true);
    return f.get(n);
  }
  static void field(Object n, String name, Object v) throws Exception {
    Field f = n.getClass().getDeclaredField(name);
    f.setAccessible(true);
    f.set(n, v);
  }
  static Object node(String type, int value) throws Exception {
    if (type.equals("ListNode"))
      return new ListNode(value);
    if (type.equals("TreeNode"))
      return new TreeNode(value);
    Class<?> c = Class.forName("Node");
    Object n;
    if (type.equals("QuadNode")) {
      n = c.getDeclaredConstructor().newInstance();
    } else {
      try {
        n = c.getDeclaredConstructor(int.class).newInstance(value);
      } catch (NoSuchMethodException e) {
        n = c.getDeclaredConstructor().newInstance();
        field(n, "val", value);
      }
    }
    return n;
  }
  interface Codec {
    Object read(Object j) throws Exception;
    Object write(Object v) throws Exception;
  }
  // Bind once from the declared Java signature. LeetCode uses both native
  // arrays and nested generic lists for the same protocol array type.
  static Codec codec(Type actual, String type) throws Exception {
    if (type.endsWith("[]")) {
      String inner = type.substring(0, type.length() - 2);
      Class<?> arrayClass = actual instanceof Class<?> ? (Class<?>) actual : null;
      if (arrayClass != null && arrayClass.isArray()) {
        Class<?> component = arrayClass.getComponentType();
        Codec child = codec(component, inner);
        return new Codec() {
          public Object read(Object j) throws Exception {
            List<Object> a = list(j);
            Object out = Array.newInstance(component, a.size());
            for (int i = 0; i < a.size(); i++) Array.set(out, i, child.read(a.get(i)));
            return out;
          }
          public Object write(Object v) throws Exception {
            List<Object> a = new ArrayList<>();
            if (v != null)
              for (int i = 0; i < Array.getLength(v); i++) a.add(child.write(Array.get(v, i)));
            return a;
          }
        };
      }
      if (actual instanceof ParameterizedType) {
        ParameterizedType t = (ParameterizedType) actual;
        Class<?> raw = (Class<?>) t.getRawType();
        check(List.class.isAssignableFrom(raw) && raw.isAssignableFrom(ArrayList.class),
            "expected array or List for " + type + ", got " + actual);
        Codec child = codec(t.getActualTypeArguments()[0], inner);
        return new Codec() {
          public Object read(Object j) throws Exception {
            List<Object> a = new ArrayList<>();
            for (Object x : list(j)) a.add(child.read(x));
            return a;
          }
          public Object write(Object v) throws Exception {
            List<Object> a = new ArrayList<>();
            if (v != null)
              for (Object x : (List<?>) v) a.add(child.write(x));
            return a;
          }
        };
      }
      throw new IllegalArgumentException(
          "expected typed array or List for " + type + ", got " + actual);
    }
    Map<String, Class<?>> primitives =
        Map.of("integer", int.class, "long", long.class, "double", double.class, "boolean",
            boolean.class, "character", char.class, "string", String.class, "void", void.class);
    Map<Class<?>, Class<?>> boxed =
        Map.of(int.class, Integer.class, long.class, Long.class, double.class, Double.class,
            boolean.class, Boolean.class, char.class, Character.class, void.class, Void.class);
    Class<?> want = primitives.get(type);
    if (want != null) {
      check(actual == want || actual == boxed.get(want), "expected " + type + ", got " + actual);
      return new Codec() {
        public Object read(Object j) {
          switch (type) {
            case "integer":
              return integer(j);
            case "long":
              return ((Number) j).longValue();
            case "double":
              return ((Number) j).doubleValue();
            case "character":
              String s = (String) j;
              check(s.length() == 1 && s.charAt(0) < 128, "character must be ASCII");
              return s.charAt(0);
            default:
              return j;
          }
        }
        public Object write(Object v) {
          if (type.equals("double") && v != null)
            check(Double.isFinite(((Number) v).doubleValue()), "non-finite double result");
          if (type.equals("character")) {
            check(v != null && (Character) v < 128, "character must be ASCII");
            return v.toString();
          }
          return v;
        }
      };
    }
    String className =
        Set.of("ListNode", "TreeNode", "NestedInteger", "Employee").contains(type) ? type : "Node";
    check(actual instanceof Class<?> && ((Class<?>) actual).getName().equals(className),
        "expected " + className + " for " + type + ", got " + actual);
    return new Codec() {
      public Object read(Object j) throws Exception {
        return readNode(type, j);
      }
      public Object write(Object v) throws Exception {
        return writeNode(type, v);
      }
    };
  }
  static Codec protocolCodec(String type) throws Exception {
    Class<?> c = switch (type) {
      case "integer" -> int.class;
      case "long" -> long.class;
      case "double" -> double.class;
      case "boolean" -> boolean.class;
      case "character" -> char.class;
      case "string" -> String.class;
      default -> Class.forName(type.equals("ListNode") || type.equals("TreeNode") ? type : "Node");
    };
    return codec(c, type);
  }
  static Object readNode(String type, Object j) throws Exception {
    List<Object> a = j instanceof List<?> ? list(j) : new ArrayList<>();
    if (type.equals("NestedInteger")) {
      if (j instanceof Number)
        return new NestedInteger(integer(j));
      NestedInteger n = new NestedInteger();
      for (Object x : a) n.add((NestedInteger) readNode(type, x));
      return n;
    }
    if (type.equals("Employee")) {
      Employee e = new Employee();
      e.id = integer(a.get(0));
      e.importance = integer(a.get(1));
      for (Object x : list(a.get(2))) e.subordinates.add(integer(x));
      return e;
    }
    if (a.isEmpty())
      return null;
    if (type.equals("ListNode")) {
      ListNode head = new ListNode(), tail = head;
      for (Object x : a) {
        tail.next = new ListNode(integer(x));
        tail = tail.next;
      }
      return head.next;
    }
    if (type.equals("TreeNode") || type.equals("NextTreeNode")) {
      if (a.get(0) == null)
        return null;
      Object root = node(type, integer(a.get(0)));
      List<Object> q = new ArrayList<>();
      q.add(root);
      int k = 1;
      for (int i = 0; k < a.size(); i++) {
        check(i < q.size(), "invalid tree");
        for (String key : new String[] {"left", "right"}) {
          if (k < a.size() && a.get(k) != null) {
            Object n = node(type, integer(a.get(k)));
            field(q.get(i), key, n);
            q.add(n);
          }
          k++;
        }
      }
      return root;
    }
    if (type.equals("GraphNode")) {
      List<Object> ns = new ArrayList<>();
      for (int i = 0; i < a.size(); i++) ns.add(node(type, i + 1));
      for (int i = 0; i < a.size(); i++)
        for (Object x : list(a.get(i)))
          list(field(ns.get(i), "neighbors")).add(ns.get(integer(x) - 1));
      return ns.get(0);
    }
    if (type.equals("NaryNode")) {
      Object root = node(type, integer(a.get(0)));
      List<Object> q = new ArrayList<>();
      q.add(root);
      int k = 2;
      for (int i = 0; k < a.size(); i++) {
        check(i < q.size(), "invalid N-ary tree");
        while (k < a.size() && a.get(k) != null) {
          Object n = node(type, integer(a.get(k++)));
          list(field(q.get(i), "children")).add(n);
          q.add(n);
        }
        k++;
      }
      return root;
    }
    if (type.equals("RandomListNode")) {
      List<Object> ns = new ArrayList<>();
      for (Object x : a) ns.add(node(type, integer(list(x).get(0))));
      for (int i = 0; i < a.size(); i++) {
        if (i + 1 < a.size())
          field(ns.get(i), "next", ns.get(i + 1));
        Object idx = list(a.get(i)).get(1);
        if (idx != null)
          field(ns.get(i), "random", ns.get(integer(idx)));
      }
      return ns.get(0);
    }
    if (type.equals("QuadNode")) {
      if (a.get(0) == null)
        return null;
      Object root = quad(a.get(0));
      List<Object> q = new ArrayList<>();
      q.add(root);
      int k = 1;
      for (int i = 0; k < a.size(); i++) {
        check(i < q.size(), "invalid quad tree");
        for (String key : quadLinks) {
          if (k < a.size() && a.get(k) != null) {
            Object n = quad(a.get(k));
            field(q.get(i), key, n);
            q.add(n);
          }
          k++;
        }
      }
      return root;
    }
    if (type.equals("MultilevelNode")) {
      Object root = null;
      List<Object> above = new ArrayList<>();
      int i = 0;
      while (i < a.size()) {
        Object parent = null;
        if (!above.isEmpty()) {
          i++;
          int k = 0;
          while (i < a.size() && a.get(i) == null) {
            i++;
            k++;
          }
          if (i == a.size())
            break;
          parent = above.get(k);
        }
        List<Object> level = new ArrayList<>();
        while (i < a.size() && a.get(i) != null) {
          Object n = node(type, integer(a.get(i++)));
          if (!level.isEmpty()) {
            field(n, "prev", level.get(level.size() - 1));
            field(level.get(level.size() - 1), "next", n);
          }
          level.add(n);
        }
        check(!level.isEmpty(), "invalid multilevel list");
        if (parent == null)
          root = level.get(0);
        else
          field(parent, "child", level.get(0));
        above = level;
      }
      return root;
    }
    throw new IllegalArgumentException("unknown node type " + type);
  }
  static final String[] quadLinks = {"topLeft", "topRight", "bottomLeft", "bottomRight"};
  static Object quad(Object j) throws Exception {
    List<Object> a = list(j);
    Object n = node("QuadNode", 0);
    field(n, "isLeaf", integer(a.get(0)) != 0);
    field(n, "val", integer(a.get(1)) != 0);
    return n;
  }
  static void trim(List<Object> a) {
    while (!a.isEmpty() && a.get(a.size() - 1) == null) a.remove(a.size() - 1);
  }
  static Object writeNode(String type, Object root) throws Exception {
    List<Object> a = new ArrayList<>();
    if (type.equals("NestedInteger")) {
      check(root != null, "null NestedInteger");
      NestedInteger n = (NestedInteger) root;
      if (n.isInteger())
        return n.getInteger();
      for (NestedInteger x : n.getList()) a.add(writeNode(type, x));
      return a;
    }
    if (type.equals("Employee")) {
      check(root != null, "null Employee");
      Employee e = (Employee) root;
      return Arrays.asList(e.id, e.importance, e.subordinates);
    }
    if (root == null)
      return a;
    Set<Object> seen = identities();
    if (type.equals("ListNode")) {
      for (ListNode n = (ListNode) root; n != null; n = n.next) {
        check(seen.size() < MAX_NODES && seen.add(n), "list has a cycle or is too large");
        a.add(n.val);
      }
      return a;
    }
    if (type.equals("TreeNode") || type.equals("QuadNode")) {
      List<Object> q = new ArrayList<>();
      q.add(root);
      for (int i = 0; i < q.size(); i++) {
        Object n = q.get(i);
        if (n == null) {
          a.add(null);
          continue;
        }
        check(seen.size() < MAX_NODES && seen.add(n), "tree has a cycle or shared node");
        if (type.equals("TreeNode")) {
          a.add(field(n, "val"));
          q.add(field(n, "left"));
          q.add(field(n, "right"));
        } else {
          boolean leaf = (Boolean) field(n, "isLeaf");
          a.add(Arrays.asList(leaf ? 1 : 0, leaf ? ((Boolean) field(n, "val") ? 1 : 0) : 1));
          for (String key : quadLinks) q.add(field(n, key));
        }
      }
      trim(a);
      return a;
    }
    if (type.equals("GraphNode")) {
      List<Object> q = new ArrayList<>();
      q.add(root);
      seen.add(root);
      Map<Integer, Object> byValue = new TreeMap<>();
      for (int i = 0; i < q.size(); i++) {
        Object n = q.get(i);
        check(q.size() <= MAX_NODES, "graph too large");
        int v = (Integer) field(n, "val");
        check(v > 0 && byValue.put(v, n) == null, "duplicate or invalid graph node value");
        for (Object c : list(field(n, "neighbors"))) {
          check(c != null, "null neighbor");
          if (seen.add(c))
            q.add(c);
        }
      }
      check((Integer) field(root, "val") == 1, "graph root must be node 1");
      for (int i = 1; i <= q.size(); i++) {
        check(byValue.containsKey(i), "graph values must be 1 to n");
        List<Object> row = new ArrayList<>();
        for (Object n : list(field(byValue.get(i), "neighbors"))) row.add(field(n, "val"));
        a.add(row);
      }
      return a;
    }
    if (type.equals("NaryNode")) {
      a.add(field(root, "val"));
      a.add(null);
      List<Object> q = new ArrayList<>();
      q.add(root);
      seen.add(root);
      for (int i = 0; i < q.size(); i++) {
        for (Object c : list(field(q.get(i), "children"))) {
          check(
              c != null && seen.size() < MAX_NODES && seen.add(c), "invalid N-ary child or cycle");
          a.add(field(c, "val"));
          q.add(c);
        }
        a.add(null);
      }
      trim(a);
      return a;
    }
    if (type.equals("RandomListNode")) {
      List<Object> ns = new ArrayList<>();
      Map<Object, Integer> index = new IdentityHashMap<>();
      for (Object n = root; n != null; n = field(n, "next")) {
        check(ns.size() < MAX_NODES && !index.containsKey(n), "random list has a cycle");
        index.put(n, ns.size());
        ns.add(n);
      }
      for (Object n : ns) {
        Object r = field(n, "random");
        check(r == null || index.containsKey(r), "random pointer outside list");
        a.add(Arrays.asList(field(n, "val"), r == null ? null : index.get(r)));
      }
      return a;
    }
    if (type.equals("NextTreeNode")) {
      List<Object> level = new ArrayList<>();
      level.add(root);
      while (!level.isEmpty()) {
        List<Object> row = new ArrayList<>(), next = new ArrayList<>();
        for (int i = 0; i < level.size(); i++) {
          Object n = level.get(i);
          check(seen.size() < MAX_NODES && seen.add(n), "next tree has a cycle");
          check(field(n, "next") == (i + 1 < level.size() ? level.get(i + 1) : null),
              "incorrect next pointer");
          row.add(field(n, "val"));
          for (String key : new String[] {"left", "right"}) {
            Object c = field(n, key);
            if (c != null)
              next.add(c);
          }
        }
        a.add(row);
        level = next;
      }
      return a;
    }
    if (type.equals("MultilevelNode")) {
      Object head = root;
      while (head != null) {
        Object child = null, prev = null;
        int at = 0, i = 0;
        for (Object n = head; n != null; prev = n, n = field(n, "next"), i++) {
          check(seen.size() < MAX_NODES && seen.add(n), "multilevel list has a cycle");
          check(field(n, "prev") == prev, "incorrect prev pointer");
          a.add(field(n, "val"));
          Object c = field(n, "child");
          if (c != null) {
            check(child == null, "multiple child levels cannot be encoded");
            child = c;
            at = i;
          }
        }
        if (child != null)
          for (int k = 0; k <= at; k++) a.add(null);
        head = child;
      }
      return a;
    }
    throw new IllegalArgumentException("unknown node type " + type);
  }
  static List<Object> nodes(Object root) throws Exception {
    List<Object> q = new ArrayList<>();
    if (root != null)
      q.add(root);
    Set<Object> seen = identities();
    for (int i = 0; i < q.size(); i++) {
      Object n = q.get(i);
      check(seen.size() < MAX_NODES && seen.add(n), "input structure has a cycle or shared node");
      for (Object c : links(n))
        if (c != null)
          q.add(c);
    }
    return q;
  }
  static List<Object> links(Object n) throws Exception {
    if (n instanceof ListNode)
      return Arrays.asList(((ListNode) n).next);
    if (n instanceof TreeNode)
      return Arrays.asList(((TreeNode) n).left, ((TreeNode) n).right);
    if (nodeVariant.equals("GraphNode"))
      return list(field(n, "neighbors"));
    if (nodeVariant.equals("NaryNode"))
      return list(field(n, "children"));
    String[] keys = switch (nodeVariant) {
      case "RandomListNode" -> new String[] {"next", "random"};
      case "NextTreeNode" -> new String[] {"left", "right", "next"};
      case "QuadNode" -> quadLinks;
      case "MultilevelNode" -> new String[] {"prev", "next", "child"};
      default -> new String[] {};
    };
    List<Object> a = new ArrayList<>();
    for (String key : keys) a.add(field(n, key));
    return a;
  }
  static Set<Object> nodeSet(Object root) throws Exception {
    Set<Object> seen = identities();
    List<Object> q = new ArrayList<>();
    if (root != null) {
      q.add(root);
      seen.add(root);
    }
    for (int i = 0; i < q.size(); i++) {
      check(q.size() <= MAX_NODES, "structure too large");
      for (Object c : links(q.get(i)))
        if (c != null && seen.add(c))
          q.add(c);
    }
    return seen;
  }
  static Object identity(Object value, List<Object> original) throws Exception {
    if (value == null)
      return null;
    for (int i = 0; i < original.size(); i++)
      if (original.get(i) == value)
        return value instanceof ListNode ? i : field(value, "val");
    throw new IllegalArgumentException("returned node is outside the input structure");
  }
  static Object ref(List<Object> original, int value) throws Exception {
    Object found = null;
    for (Object n : original)
      if ((Integer) field(n, "val") == value) {
        check(found == null, "ambiguous node value");
        found = n;
      }
    check(found != null, "node value not found");
    return found;
  }

  static final class Call {
    Executable executable;
    Codec[] inputs;
    Codec output;
    Call(Executable e, List<Object> params, String returns) throws Exception {
      executable = e;
      e.setAccessible(true);
      Type[] ts = e.getGenericParameterTypes();
      check(ts.length == params.size(), "parameter count mismatch");
      inputs = new Codec[ts.length];
      for (int i = 0; i < ts.length; i++) inputs[i] = codec(ts[i], str(map(params.get(i)), "type"));
      if (e instanceof Method)
        output = codec(((Method) e).getGenericReturnType(), returns);
    }
    Object[] decode(List<Object> args) throws Exception {
      Object[] a = new Object[inputs.length];
      for (int i = 0; i < a.length; i++) a[i] = inputs[i].read(args.get(i));
      return a;
    }
    Object invoke(Object obj, Object[] args, boolean timed) throws Throwable {
      if (timed)
        start();
      try {
        return executable instanceof Method ? ((Method) executable).invoke(obj, args)
                                            : ((Constructor<?>) executable).newInstance(args);
      } catch (InvocationTargetException e) {
        throw e.getCause();
      } finally {
        if (timed)
          stop();
      }
    }
  }
  static Call bind(Class<?> cl, String name, List<Object> params, String returns) throws Exception {
    List<Executable> es = new ArrayList<>();
    if (name == null)
      es.addAll(Arrays.asList(cl.getDeclaredConstructors()));
    else
      for (Method m : cl.getDeclaredMethods())
        if (m.getName().equals(name) && !m.isBridge() && !m.isSynthetic())
          es.add(m);
    Call selected = null;
    String why = "no matching method";
    for (Executable e : es) {
      if (e.getParameterCount() != params.size())
        continue;
      try {
        Call c = new Call(e, params, returns);
        check(selected == null, "ambiguous overload: " + name);
        selected = c;
      } catch (IllegalArgumentException ex) {
        why = ex.getMessage();
        if (why.startsWith("ambiguous"))
          throw ex;
      }
    }
    check(selected != null,
        "signature " + cl.getName() + "." + (name == null ? "<init>" : name) + ": " + why);
    return selected;
  }
  // Reflection metadata and codecs are shared across cases; solution objects
  // and decoded inputs are recreated. No per-case class loader or forced GC.
  static final class Binding {
    Map<String, Object> spec, fn, judge, cl, round;
    List<Object> params;
    Call constructor, method, encoder, decoder;
    Map<String, Call> methods = new HashMap<>();
    Binding(Map<String, Object> s) throws Exception {
      spec = s;
      judge = map(s.get("judge"));
      nodeVariant = str(s, "node");
      if (str(s, "kind").equals("function")) {
        fn = map(s.get("function"));
        params = list(fn.get("params"));
        Class<?> c = Class.forName("Solution", false, HCRuntime.class.getClassLoader());
        constructor = bind(c, null, List.of(), "");
        method = bind(c, str(fn, "name"), params, str(fn, "returns"));
      } else {
        cl = map(s.get("class"));
        Class<?> c = Class.forName(str(cl, "name"), false, HCRuntime.class.getClassLoader());
        constructor = bind(c, null, list(cl.get("constructor")), "");
        for (Object x : list(cl.get("methods"))) {
          Map<String, Object> m = map(x);
          methods.put(
              str(m, "name"), bind(c, str(m, "name"), list(m.get("params")), str(m, "returns")));
        }
        if (judge.get("round_trip") != null) {
          round = map(judge.get("round_trip"));
          encoder = methods.get(str(round, "encode"));
          decoder = methods.get(str(round, "decode"));
        }
      }
    }
    Object call(Map<String, Object> input) throws Throwable {
      if (fn != null)
        return function(input);
      if (round != null) {
        Map<String, Object> enc = null;
        for (Object x : list(cl.get("methods")))
          if (str(map(x), "name").equals(str(round, "encode")))
            enc = map(x);
        String key = str(map(list(enc.get("params")).get(0)), "name");
        Object arg = encoder.inputs[0].read(input.get(key));
        Object first = constructor.invoke(null, new Object[0], true);
        Object text = encoder.invoke(first, new Object[] {arg}, true);
        Object second = str(round, "instances").equals("same")
            ? first
            : constructor.invoke(null, new Object[0], true);
        return decoder.output.write(decoder.invoke(second, new Object[] {text}, true));
      }
      List<Object> ops = list(input.get("ops")), args = list(input.get("args")),
                   out = new ArrayList<>();
      Object obj = constructor.invoke(null, constructor.decode(list(args.get(0))), true);
      out.add(null);
      for (int i = 1; i < ops.size(); i++) {
        Call c = methods.get(ops.get(i));
        check(c != null, "unknown method " + ops.get(i));
        Object[] a = c.decode(list(args.get(i)));
        Object result = c.invoke(obj, a, true);
        out.add(c.output.write(result));
      }
      return out;
    }
    Object function(Map<String, Object> input) throws Throwable {
      Map<String, Object> values = new HashMap<>();
      Map<String, Codec> codecs = new HashMap<>();
      Map<String, List<Object>> original = new HashMap<>();
      List<Object> all = new ArrayList<>(params);
      all.addAll(list(fn.get("hidden")));
      for (int i = 0; i < all.size(); i++) {
        Map<String, Object> p = map(all.get(i));
        String name = str(p, "name"), type = str(p, "type");
        Codec c = i < params.size() ? method.inputs[i] : protocolCodec(type);
        codecs.put(name, c);
        if (p.containsKey("ref") || p.containsKey("copy"))
          continue;
        Object v = c.read(input.get(name));
        values.put(name, v);
        if (type.equals("ListNode") || type.equals("TreeNode"))
          original.put(name, nodes(v));
      }
      for (Object x : params) {
        Map<String, Object> p = map(x);
        String name = str(p, "name");
        if (p.containsKey("cycle")) {
          int pos = (Integer) values.get(p.get("cycle"));
          if (pos >= 0) {
            List<Object> ns = original.get(name);
            ((ListNode) ns.get(ns.size() - 1)).next = (ListNode) ns.get(pos);
          }
        }
        if (p.containsKey("join")) {
          Map<String, Object> j = map(p.get("join"));
          int skip = (Integer) values.get(j.get("skip")), at = (Integer) values.get(j.get("at"));
          List<Object> a = original.get(str(j, "list")), b = original.get(name);
          List<Object> ns = new ArrayList<>(b.subList(0, skip));
          ns.addAll(a.subList(at, a.size()));
          if (skip > 0)
            ((ListNode) ns.get(skip - 1)).next = skip < ns.size() ? (ListNode) ns.get(skip) : null;
          values.put(name, ns.isEmpty() ? null : ns.get(0));
          original.put(name, ns);
        }
      }
      for (Object x : params) {
        Map<String, Object> p = map(x);
        String name = str(p, "name");
        if (p.containsKey("copy")) {
          Codec c = codecs.get(name);
          Object v = c.read(c.write(values.get(p.get("copy"))));
          values.put(name, v);
          original.put(name, nodes(v));
        }
      }
      for (Object x : params) {
        Map<String, Object> p = map(x);
        String name = str(p, "name");
        if (p.containsKey("ref"))
          values.put(name, ref(original.get(p.get("ref")), integer(input.get(name))));
      }
      Set<Object> before =
          fn.containsKey("returns_copy") ? nodeSet(values.get(fn.get("returns_copy"))) : null;
      Object[] args = new Object[params.size()];
      for (int i = 0; i < args.length; i++) args[i] = values.get(map(params.get(i)).get("name"));
      Object obj = constructor.invoke(null, new Object[0], false);
      Object result = method.invoke(obj, args, true);
      if (before != null)
        for (Object n : nodeSet(result))
          check(!before.contains(n), "returned copy shares a node with input");
      String output = str(judge, "output");
      if (!output.isEmpty()) {
        Object encoded = codecs.get(output).write(values.get(output));
        if (!str(judge, "output_length").isEmpty()) {
          int k = integer(result);
          List<Object> a = list(encoded);
          check(k >= 0 && k <= a.size(), "returned length outside output array");
          return new ArrayList<>(a.subList(0, k));
        }
        return encoded;
      }
      if (fn.containsKey("returns_ref"))
        return identity(result, original.get(fn.get("returns_ref")));
      return method.output.write(result);
    }
  }
  // The single solution thread writes the clock under a sequence counter;
  // the watchdog reads a consistent total without locking every method call.
  static volatile long elapsed, started;
  static volatile int generation;
  static void start() {
    generation++;
    started = System.nanoTime();
    generation++;
  }
  static void stop() {
    long now = System.nanoTime();
    generation++;
    long s = started;
    started = 0;
    if (s != 0)
      elapsed += now - s;
    generation++;
  }
  static long spent() {
    for (;;) {
      int g = generation;
      if ((g & 1) != 0)
        continue;
      long e = elapsed, s = started;
      if (g == generation)
        return e + (s == 0 ? 0 : System.nanoTime() - s);
    }
  }
  static final Object resultLock = new Object();
  static int currentID;
  static long caseStart;
  static boolean active, done;
  static String error(Throwable e) {
    StringBuilder b = new StringBuilder(e.toString());
    int n = 0;
    for (StackTraceElement f : e.getStackTrace())
      if ("Solution.java".equals(f.getFileName()) && n++ < 12)
        b.append("\n    at ").append(f);
    return b.toString();
  }
  static void run(String specification) throws Exception {
    Binding binding = new Binding(map(HCJson.parse(specification)));
    List<Object> cases = list(
        HCJson.parse(Files.readString(Path.of(System.getenv("HC_CASES")), StandardCharsets.UTF_8)));
    try (PrintWriter out = new PrintWriter(
             Files.newBufferedWriter(Path.of(System.getenv("HC_OUT")), StandardCharsets.UTF_8))) {
      String setting = System.getenv("HC_TIME_LIMIT_NS");
      long limit = setting == null ? 0 : Long.parseLong(setting);
      if (setting != null) {
        Thread watchdog = new Thread(() -> {
          try {
            while (true) {
              synchronized (resultLock) {
                if (done)
                  return;
                long t = spent();
                if (active && t > limit) {
                  Map<String, Object> r = new LinkedHashMap<>();
                  r.put("id", currentID);
                  r.put("time_limit", true);
                  r.put("elapsed_ns", t - caseStart);
                  out.println(HCJson.write(r));
                  out.flush();
                  Runtime.getRuntime().halt(0);
                }
              }
              Thread.sleep(1);
            }
          } catch (InterruptedException ignored) {
          }
        }, "hc-time-limit");
        watchdog.setDaemon(true);
        watchdog.start();
      }
      for (Object x : cases) {
        Map<String, Object> c = map(x);
        int id = integer(c.get("id"));
        synchronized (resultLock) {
          currentID = id;
          caseStart = elapsed;
          active = true;
        }
        Map<String, Object> r = new LinkedHashMap<>();
        r.put("id", id);
        boolean fatal = false;
        try {
          r.put("output", binding.call(map(c.get("input"))));
        } catch (Throwable e) {
          stop();
          r.put("error", error(e));
          fatal = e instanceof VirtualMachineError || e instanceof ThreadDeath
              || e instanceof LinkageError;
        }
        synchronized (resultLock) {
          long t = elapsed;
          r.put("elapsed_ns", t - caseStart);
          if (fatal)
            r.put("fatal", true);
          if (setting != null && t > limit) {
            r.remove("output");
            r.put("time_limit", true);
            fatal = true;
          }
          out.println(HCJson.write(r));
          out.flush();
          active = false;
        }
        if (fatal)
          break;
      }
      synchronized (resultLock) {
        done = true;
      }
    }
    // The batch owns this JVM, including non-daemon threads started by solutions.
    // Result output is closed before ending it; no shutdown hook can delay a run.
    Runtime.getRuntime().halt(0);
  }
}
