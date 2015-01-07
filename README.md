# hlsvalidator

Validate and optionally download a HLS stream. If there is a manifest with multiple bitrate playlists, *all* playlists will be processed.

Usage: hlsvalidator [-l=bool (localtime)] [-v=bool (verbose output)] [-t duration] [-ua user-agent] [-d destination] hls-url

Options:

* Localtime: Use local time to track duration instead of supplied metadata.

* Duration: Download destination (file).

* User agent: User-Agent for HTTP client

* Destination: Verbose output
