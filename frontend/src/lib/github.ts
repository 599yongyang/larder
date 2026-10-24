// The official GitHub avatar endpoint is always tried first. The remaining
// services are image proxies (not code mirrors) that also cache GitHub avatars
// and work when direct GitHub access is slow or unavailable.
export function githubAvatarUrls(owner: string): string[] {
  const official = `https://github.com/${encodeURIComponent(owner)}.png?size=64`
  const encoded = encodeURIComponent(official)
  return [
    official,
    `https://wsrv.nl/?url=${encoded}&w=64&h=64&fit=cover&output=png`,
    `https://unavatar.io/github/${encodeURIComponent(owner)}`,
  ]
}
