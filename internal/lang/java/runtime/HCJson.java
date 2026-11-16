import java.util.*;

// JSON transport only; comparison and protocol range validation belong to hc.
// Integers are parsed directly as long, never through double.
final class HCJson {
  private final String text;
  private int at;
  private HCJson(String text) {
    this.text = text;
  }
  static Object parse(String text) {
    HCJson p = new HCJson(text);
    Object v = p.value();
    p.space();
    if (p.at != text.length())
      throw new IllegalArgumentException("trailing JSON data");
    return v;
  }
  private void space() {
    while (at < text.length() && Character.isWhitespace(text.charAt(at))) at++;
  }
  private char take() {
    if (at == text.length())
      throw new IllegalArgumentException("unexpected end of JSON");
    return text.charAt(at++);
  }
  private void expect(char c) {
    space();
    if (take() != c)
      throw new IllegalArgumentException("expected " + c);
  }
  private Object value() {
    space();
    char c = take();
    if (c == '"')
      return string();
    if (c == '[') {
      List<Object> a = new ArrayList<>();
      space();
      if (at < text.length() && text.charAt(at) == ']') {
        at++;
        return a;
      }
      do {
        a.add(value());
        space();
        c = take();
        if (c == ']')
          return a;
        if (c != ',')
          throw new IllegalArgumentException("expected comma");
      } while (true);
    }
    if (c == '{') {
      Map<String, Object> m = new LinkedHashMap<>();
      space();
      if (at < text.length() && text.charAt(at) == '}') {
        at++;
        return m;
      }
      do {
        expect('"');
        String k = string();
        expect(':');
        m.put(k, value());
        space();
        c = take();
        if (c == '}')
          return m;
        if (c != ',')
          throw new IllegalArgumentException("expected comma");
      } while (true);
    }
    for (String literal : new String[] {"true", "false", "null"}) {
      if (c == literal.charAt(0) && text.startsWith(literal, at - 1)) {
        at += literal.length() - 1;
        if (literal.equals("null"))
          return null;
        return literal.equals("true");
      }
    }
    int start = at - 1;
    while (at < text.length() && "0123456789.eE+-".indexOf(text.charAt(at)) >= 0) at++;
    String n = text.substring(start, at);
    if (n.indexOf('.') >= 0 || n.indexOf('e') >= 0 || n.indexOf('E') >= 0)
      return Double.valueOf(n);
    try {
      return Long.valueOf(n);
    } catch (NumberFormatException large) {
      // A protocol double may be written as an integer token outside int64.
      // Keep integers exact until the signature-specific codec converts it.
      return new java.math.BigInteger(n);
    }
  }
  private String string() {
    StringBuilder b = new StringBuilder();
    while (true) {
      char c = take();
      if (c == '"')
        return b.toString();
      if (c == '\\') {
        c = take();
        switch (c) {
          case '"':
          case '\\':
          case '/':
            b.append(c);
            break;
          case 'b':
            b.append('\b');
            break;
          case 'f':
            b.append('\f');
            break;
          case 'n':
            b.append('\n');
            break;
          case 'r':
            b.append('\r');
            break;
          case 't':
            b.append('\t');
            break;
          case 'u':
            int n = 0;
            for (int i = 0; i < 4; i++) {
              int d = Character.digit(take(), 16);
              if (d < 0)
                throw new IllegalArgumentException("bad unicode escape");
              n = n * 16 + d;
            }
            b.append((char) n);
            break;
          default:
            throw new IllegalArgumentException("bad JSON escape");
        }
      } else {
        if (c < 32)
          throw new IllegalArgumentException("control character in JSON");
        b.append(c);
      }
    }
  }
  static String write(Object v) {
    StringBuilder b = new StringBuilder();
    write(b, v);
    return b.toString();
  }
  private static void write(StringBuilder b, Object v) {
    if (v == null) {
      b.append("null");
      return;
    }
    if (v instanceof String || v instanceof Character) {
      b.append('"');
      String s = v.toString();
      for (int i = 0; i < s.length(); i++) {
        char c = s.charAt(i);
        switch (c) {
          case '"':
            b.append("\\\"");
            break;
          case '\\':
            b.append("\\\\");
            break;
          case '\n':
            b.append("\\n");
            break;
          case '\r':
            b.append("\\r");
            break;
          case '\t':
            b.append("\\t");
            break;
          default:
            if (c < 32 || Character.isSurrogate(c))
              b.append(String.format("\\u%04x", (int) c));
            else
              b.append(c);
        }
      }
      b.append('"');
      return;
    }
    if (v instanceof Double && !Double.isFinite((Double) v)
        || v instanceof Float && !Float.isFinite((Float) v))
      throw new IllegalArgumentException("non-finite output");
    if (v instanceof Number || v instanceof Boolean) {
      b.append(v);
      return;
    }
        if(v instanceof Map<?,?>){
          b.append('{');
          boolean first = true;for(var e:((Map<?,?>)v).entrySet()){
            if (!first)
              b.append(',');
            first = false;
            write(b, e.getKey());
            b.append(':');
            write(b, e.getValue());
          }
          b.append('}');
          return;
        }
        if (v instanceof Iterable<?>) {
          b.append('[');
          boolean first = true;
          for (Object x : (Iterable<?>) v) {
            if (!first)
              b.append(',');
            first = false;
            write(b, x);
          }
          b.append(']');
          return;
        }
        throw new IllegalArgumentException("not a JSON value: " + v.getClass().getName());
  }
}
