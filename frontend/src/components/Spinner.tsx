interface SpinnerProps {
  label?: string
}

export default function Spinner({ label }: SpinnerProps) {
  return (
    <div className="center" style={{ padding: '36px 0', gap: 12 }}>
      <div className="spinner" />
      {label && <span className="faint" style={{ fontSize: 13 }}>{label}</span>}
    </div>
  )
}
