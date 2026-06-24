// A lightweight, auto-dismissing notification used for non-blocking feedback
// (e.g. manual update check: "已是最新版本"). It floats at the bottom-center.
export function Toast({ message }: { message: string | null }) {
  if (!message) return null
  return (
    <div className="toast" role="status">{message}</div>
  )
}
