const urls = new Map<string, string>();
let nextId = 0;
function encode(longUrl: string): string {
  const id = "https://hc.local/" + nextId++;
  urls.set(id, longUrl);
  return id;
}
function decode(shortUrl: string): string {
  return urls.get(shortUrl);
}
