import { useEffect, useState } from "react";
import { formatCountdown } from "@/utils/format";

interface CapsuleCountdownProps {
  unlockAt: string;
  // onUnlock：倒计时归零时回调，父组件可重新拉取以展示已解锁内容
  onUnlock?: () => void;
}

// CapsuleCountdown 胶囊解锁倒计时，跨「我封存的 / 收到的」列表复用。
export default function CapsuleCountdown({ unlockAt, onUnlock }: CapsuleCountdownProps) {
  const [remaining, setRemaining] = useState<string | null>(formatCountdown(unlockAt));

  useEffect(() => {
    const timer = setInterval(() => {
      const next = formatCountdown(unlockAt);
      setRemaining(next);
      if (next === null) {
        clearInterval(timer);
        onUnlock?.();
      }
    }, 1000);
    return () => clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [unlockAt]);

  if (remaining === null) return null;
  return (
    <p className="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700">
      ⏳ 距解锁还有 <span className="font-semibold">{remaining}</span>
    </p>
  );
}
