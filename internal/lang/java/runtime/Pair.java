package javafx.util;
import java.io.Serializable;
import java.util.Objects;
/** The small immutable Pair API supplied by LeetCode; no JavaFX dependency. */
public class Pair<K, V> implements Serializable {
  private final K key;
  private final V value;
  public Pair(K key, V value) {
    this.key = key;
    this.value = value;
  }
  public K getKey() {
    return key;
  }
  public V getValue() {
    return value;
  }
  @Override
  public boolean equals(Object other) {
    return other instanceof Pair < ?, ? > &&Objects.equals(key, ((Pair < ?, ? >) other).key)
                                            && Objects.equals(value, ((Pair < ?, ? >) other).value);
  }
  @Override
  public int hashCode() {
    return Objects.hashCode(key) * 13 + Objects.hashCode(value);
  }
  @Override
  public String toString() {
    return key + "=" + value;
  }
}
