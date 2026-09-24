import type { Capsule } from "@/api/capsule";
import CapsuleCountdown from "@/components/CapsuleCountdown";
import StatusBadge from "@/components/StatusBadge";
import { formatDate } from "@/utils/format";

interface CapsuleCardProps {
  capsule: Capsule;
  // onOpen：点击卡片查看详情（正文/图片/语音/回信）
  onOpen: (capsule: Capsule) => void;
  onDelete?: (capsule: Capsule) => void;
  onReload?: () => void;
}

// CapsuleCard 胶囊卡片：「我封存的」与「收到的」共用，按 role 与 status 控制可见内容。
export default function CapsuleCard({ capsule, onOpen, onDelete, onReload }: CapsuleCardProps) {
  const isOwner = capsule.role === "owner";
  const locked = capsule.status === "locked";

  return (
    <div className={`card space-y-3 ${locked ? "border-amber-100" : "border-emerald-200"}`}>
      <div className="flex items-center justify-between">
        <span className="text-2xl">{locked ? "🔒" : "📬"}</span>
        <StatusBadge status={capsule.status} kind="capsule" />
      </div>

      <div>
        <h3 className="font-semibold text-gray-800">{capsule.title}</h3>
        <p className="mt-1 text-xs text-gray-400">
          {isOwner
            ? capsule.recipient_username
              ? `共同开启：${capsule.recipient_username}`
              : "仅自己可见"
            : `来自 ${capsule.owner_nickname || capsule.owner_username || "匿名朋友"}`}
        </p>
        <p className="mt-0.5 text-xs text-gray-400">解锁时间 {formatDate(capsule.unlock_at)}</p>
      </div>

      {locked ? (
        <CapsuleCountdown unlockAt={capsule.unlock_at} onUnlock={onReload} />
      ) : (
        <div className="animate-unlock space-y-2">
          <p className="line-clamp-3 text-sm text-gray-600">{capsule.content}</p>
          {capsule.image_urls.length > 0 && (
            <div className="flex gap-2 overflow-hidden">
              {capsule.image_urls.slice(0, 3).map((url) => (
                // eslint-disable-next-line @next/next/no-img-element
                <img key={url} src={url} alt="胶囊图片" className="h-16 w-16 rounded-lg object-cover" />
              ))}
            </div>
          )}
          {capsule.audio_url && (
            <audio controls className="w-full" src={capsule.audio_url}>
              您的浏览器不支持音频播放
            </audio>
          )}
          {capsule.reply && (
            <p className="rounded-lg bg-violet-50 px-3 py-2 text-xs text-violet-700">
              ✉️ {isOwner ? "收件人" : "你"}的回信：{capsule.reply.status === "withdrawn" ? "已撤回" : "已送达"}
            </p>
          )}
        </div>
      )}

      <div className="flex items-center justify-between">
        <button className="text-xs text-purple-500 hover:text-purple-700" onClick={() => onOpen(capsule)}>
          {locked ? "查看封面" : "查看详情"}
        </button>
        {isOwner && onDelete && (
          <button className="text-xs text-red-400 hover:text-red-600" onClick={() => onDelete(capsule)}>
            删除
          </button>
        )}
      </div>
    </div>
  );
}
