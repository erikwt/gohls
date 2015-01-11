# hlsvalidator

Validate and optionally download an HLS stream. If there is a master manifest with multiple bitrate playlists, *all* playlists will be processed.

Releases
--------
* [0.0.1 - Jan 8 2015](https://github.com/erikwt/hlsvalidator/releases/tag/hlsvalidator_0.0.1)

Usage
-----
hlsvalidator [-l (use local time)] [-v (verbose output)] [-t duration] [-ua user-agent] [-d destination] hls-url

Options
-------
* Localtime: Use local time to track duration instead of supplied metadata.
* Duration: Download destination (directory).
* User agent: User-Agent for HTTP client
* Destination: Verbose output
