import { useEffect, useState } from "react";

// useNow：每秒刷新一次当前时间，供时光胶囊剩余时间倒计时使用。
export function useNow(intervalMs = 1000): Date {
  const [now, setNow] = useState<Date>(() => new Date());
  useEffect(() => {
    const timer = setInterval(() => setNow(new Date()), intervalMs);
    return () => clearInterval(timer);
  }, [intervalMs]);
  return now;
}

// formatRemain：计算目标时间距现在的剩余时间文本；已到期返回“已到期”。
export function formatRemain(target: string | Date, now: Date): string {
  const targetTime = typeof target === "string" ? new Date(target) : target;
  const diff = targetTime.getTime() - now.getTime();
  if (diff <= 0) return "已到期";
  const totalSeconds = Math.floor(diff / 1000);
  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  if (days > 0) return `${days}天 ${pad(hours)}:${pad(minutes)}:${pad(seconds)}`;
  return `${pad(hours)}:${pad(minutes)}:${pad(seconds)}`;
}
