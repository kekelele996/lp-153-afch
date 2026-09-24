import { useEffect, useState } from "react";
import type { Capsule } from "@/api/capsule";
import { capsuleApi } from "@/api/capsule";
import CapsuleCountdown from "@/components/CapsuleCountdown";
import ConfirmDialog from "@/components/ConfirmDialog";
import { useToast } from "@/components/Toast";
import { formatDate } from "@/utils/format";

interface CapsuleDetailModalProps {
  capsuleId: number | null;
  onClose: () => void;
  onChanged: () => void;
}

// CapsuleDetailModal 胶囊详情：解锁前双方只见标题与倒计时；
// 解锁后双方阅读正文/图片/语音；收件人回信一次；主人撤回回信并保留可见。
export default function CapsuleDetailModal({ capsuleId, onClose, onChanged }: CapsuleDetailModalProps) {
  const toast = useToast();
  const [capsule, setCapsule] = useState<Capsule | null>(null);
  const [replyText, setReplyText] = useState("");
  const [sending, setSending] = useState(false);
  const [confirmWithdraw, setConfirmWithdraw] = useState(false);

  const load = async () => {
    if (capsuleId === null) return;
    try {
      const data = await capsuleApi.detail(capsuleId);
      setCapsule(data);
    } catch (e) {
      toast.show((e as Error).message, "error");
      onClose();
    }
  };

  useEffect(() => {
    setCapsule(null);
    setReplyText("");
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [capsuleId]);

  if (capsuleId === null) return null;

  const isOwner = capsule?.role === "owner";
  const locked = capsule?.status === "locked";
  const reply = capsule?.reply;

  const sendReply = async () => {
    if (!replyText.trim()) {
      toast.show("回信内容不能为空", "error");
      return;
    }
    setSending(true);
    try {
      await capsuleApi.reply(capsuleId, replyText.trim());
      toast.show("回信已发出，发出后不可修改 ✉️");
      setReplyText("");
      await load();
      onChanged();
    } catch (e) {
      toast.show((e as Error).message, "error");
    } finally {
      setSending(false);
    }
  };

  const withdraw = async () => {
    try {
      await capsuleApi.withdrawReply(capsuleId);
      toast.show("回信已撤回");
      setConfirmWithdraw(false);
      await load();
      onChanged();
    } catch (e) {
      toast.show((e as Error).message, "error");
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4" onClick={onClose}>
      <div className="max-h-[85vh] w-full max-w-xl space-y-4 overflow-y-auto rounded-2xl bg-white p-6 shadow-xl" onClick={(e) => e.stopPropagation()}>
        {!capsule ? (
          <p className="py-10 text-center text-sm text-gray-400">加载中...</p>
        ) : (
          <>
            <div className="flex items-start justify-between gap-4">
              <div>
                <h3 className="text-lg font-semibold text-gray-800">{capsule.title}</h3>
                <p className="mt-1 text-xs text-gray-400">
                  {isOwner
                    ? capsule.recipient_username
                      ? `共同开启人：${capsule.recipient_username}`
                      : "仅自己可见的胶囊"
                    : `来自 ${capsule.owner_nickname || capsule.owner_username || "朋友"}`}
                </p>
                <p className="mt-0.5 text-xs text-gray-400">解锁时间 {formatDate(capsule.unlock_at)}</p>
              </div>
              <span className="text-2xl">{locked ? "🔒" : "📬"}</span>
            </div>

            {locked ? (
              <div className="space-y-3">
                <CapsuleCountdown unlockAt={capsule.unlock_at} onUnlock={load} />
                <p className="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700">
                  🔐 未到解锁时间，正文、图片与语音暂不可见
                </p>
              </div>
            ) : (
              <div className="animate-unlock space-y-4">
                <p className="whitespace-pre-wrap text-sm leading-6 text-gray-700">{capsule.content}</p>
                {capsule.image_urls.length > 0 && (
                  <div className="grid grid-cols-3 gap-2">
                    {capsule.image_urls.map((url) => (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img key={url} src={url} alt="胶囊图片" className="h-28 w-full rounded-lg object-cover" />
                    ))}
                  </div>
                )}
                {capsule.audio_url && (
                  <audio controls className="w-full" src={capsule.audio_url}>
                    您的浏览器不支持音频播放
                  </audio>
                )}

                {/* 回信区 */}
                {reply ? (
                  <div className="space-y-2 rounded-xl border border-violet-100 bg-violet-50/60 p-4">
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-medium text-violet-700">
                        ✉️ {isOwner ? "收件人的回信" : "我的回信"}
                      </span>
                      <span className="text-xs text-violet-400">{formatDate(reply.created_at)}</span>
                    </div>
                    {reply.status === "withdrawn" ? (
                      isOwner ? (
                        <>
                          <p className="whitespace-pre-wrap text-sm text-gray-500 line-through">{reply.content}</p>
                          <p className="text-xs text-gray-400">你已于 {formatDate(reply.withdrawn_at)} 撤回（仅你仍可查看原文）</p>
                        </>
                      ) : (
                        <p className="text-xs text-gray-400">这封回信已被对方撤回，内容不可见</p>
                      )
                    ) : (
                      <>
                        <p className="whitespace-pre-wrap text-sm leading-6 text-gray-700">{reply.content}</p>
                        <p className="text-xs text-gray-400">
                          {isOwner ? "回信仅可由你撤回" : "回信已发出，不能修改"}
                        </p>
                      </>
                    )}
                    {isOwner && reply.status === "active" && (
                      <div className="flex justify-end">
                        <button className="text-xs text-red-400 hover:text-red-600" onClick={() => setConfirmWithdraw(true)}>
                          撤回这封回信
                        </button>
                      </div>
                    )}
                  </div>
                ) : (
                  !isOwner && (
                    <div className="space-y-2 rounded-xl border border-violet-100 p-4">
                      <label className="text-sm font-medium text-violet-700">写一封回信（仅一次，发出不可改）</label>
                      <textarea
                        className="input min-h-[80px]"
                        value={replyText}
                        onChange={(e) => setReplyText(e.target.value)}
                        placeholder="读完胶囊，想对 TA 说些什么..."
                      />
                      <div className="flex justify-end">
                        <button className="btn-primary px-4 py-1.5 text-sm" disabled={sending} onClick={sendReply}>
                          {sending ? "发送中..." : "发出回信"}
                        </button>
                      </div>
                    </div>
                  )
                )}
              </div>
            )}

            <div className="flex justify-end">
              <button className="btn-secondary" onClick={onClose}>关闭</button>
            </div>
          </>
        )}
      </div>

      <ConfirmDialog
        open={confirmWithdraw}
        title="撤回回信"
        description="撤回后收件人将看不到这封回信，你仍可在详情中查看原文。确定撤回吗？"
        confirmText="撤回"
        danger
        onConfirm={withdraw}
        onCancel={() => setConfirmWithdraw(false)}
      />
    </div>
  );
}
