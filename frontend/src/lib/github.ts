// The official GitHub avatar endpoint is always tried first. The remaining
// services are image proxies (not code mirrors) that also cache GitHub avatars
// and work when direct GitHub access is slow or unavailable.
export function githubAvatarUrls(owner: string): string[] {
  const official = `https://github.com/${encodeURIComponent(owner)}.png?size=64`
  const encoded = encodeURIComponent(official)
  // GitHub direct is often slow or blocked from the desktop's network. Start
  // with an image proxy and fall back to the official endpoint.
  return [
    `https://wsrv.nl/?url=${encoded}&w=64&h=64&fit=cover&output=png`,
    `https://unavatar.io/github/${encodeURIComponent(owner)}`,
    official,
  ]
}
