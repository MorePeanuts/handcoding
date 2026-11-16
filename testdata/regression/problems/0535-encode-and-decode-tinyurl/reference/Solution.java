import java.util.*;

class Codec {
  final Map<String, String> values = new HashMap<>();
  public String encode(String longUrl) {
    String key = "http://tinyurl.com/" + values.size();
    values.put(key, longUrl);
    return key;
  }
  public String decode(String shortUrl) {
    return values.get(shortUrl);
  }
}
