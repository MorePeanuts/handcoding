class Codec:
    """Numbers the URLs in the order they are encoded."""

    def __init__(self):
        self.urls: list[str] = []
        self.ids: dict[str, int] = {}

    def encode(self, longUrl: str) -> str:
        if longUrl not in self.ids:
            self.ids[longUrl] = len(self.urls)
            self.urls.append(longUrl)
        return "http://tinyurl.com/" + str(self.ids[longUrl])

    def decode(self, shortUrl: str) -> str:
        return self.urls[int(shortUrl.rsplit("/", 1)[1])]
