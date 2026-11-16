"use strict";
const urls = new Map();
let nextId = 0;
function encode(longUrl) {
  const id = "https://hc.local/" + nextId++;
  urls.set(id, longUrl);
  return id;
}
function decode(shortUrl) {
  return urls.get(shortUrl);
}
