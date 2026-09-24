import { useCallback, useEffect, useState } from "react";
import type { Capsule } from "@/api/capsule";
import { capsuleApi } from "@/api/capsule";
import { uploadApi } from "@/api/upload";
import ConfirmDialog from "@/components/ConfirmDialog";
import EmptyState from "@/components/EmptyState";
import RequireAuth from "@/components/RequireAuth";
import StatusBadge from "@/components/StatusBadge";
import { useToast } from "@/components/Toast";
import { useAuth } from "@/hooks/useAuth";
import { formatDate } from "@/utils/format";
import { formatRemain, useNow } from "@/hooks/useNow";

type TabKey = "mine" | "received";

export default function Capsules() {
  const { user, isAuthed } = useAuth();
  const toast = useToast();
  const now = useNow();
  const [tab, setTab] = useState<TabKey>("mine");
  const [mine, setMine] = useState<Capsule[]>([]);
  const [received, setReceived] = useState<Capsule[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [detailId, setDetailId] = useState<number | null>(null);
  const [detail, setDetail] = useState<Capsule | null>(null);
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const [withdrawId, setWithdrawId] = useState<number | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [mineData, receivedData] = await Promise.all([
        capsuleApi.mine({ page: 1, page_size: 50 }),
        capsuleApi.received({ page: 1, page_size: 50 }),
      ]);
      setMine(mineData.items);
      setReceived(receivedData.items);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (isAuthed()) load();
  }, [isAuthed, load]);

  const loadDetail = useCallback(async (id: number) => {
    const data = await capsuleApi.detail(id);
    setDetail(data);
  }, []);

  useEffect(() => {
    if (detailId !== null) loadDetail(detailId);
  }, [detailId, loadDetail]);

  const openDetail = (id: number) => {
    setDetailId(id);
    setDetail(null);
  };

  const closeDetail = () => {
    setDetailId(null);
    setDetail(null);
  };

  const remove = async () => {
    if (!deleteId) return;
    try {
      await capsuleApi.remove(deleteId);
      toast.show("胶囊已删除");
      setDeleteId(null);
      closeDetail();
      load();
    } catch (e) {
      toast.show((e as Error).message, "error");
    }
  };

  const withdraw = async () => {
    if (!withdrawId) return;
    try {
      await capsuleApi.withdrawReply(withdrawId);
      toast.show("回信已撤回");
      setWithdrawId(null);
      await loadDetail(withdrawId);
      load();
    } catch (e) {
      toast.show((e as Error).message, "error");
    }
  };

  const onReplied = async (id: number) => {
    await loadDetail(id);
    load();
  };

  const list = tab === "mine" ? mine : received;

  return (
    <RequireAuth>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-purple-700">📦 时光胶囊</h1>
            <p className="mt-1 text-sm text-gray-500">
              写给未来的自己，也可以约一位朋友到期一起打开
            </p>
          </div>
          <button className="btn-primary" onClick={() => setShowCreate(true)}>封存新胶囊</button>
        </div>

        <div className="flex gap-2 border-b border-gray-200">
          <button
            className={`-mb-px border-b-2 px-4 py-2 text-sm font-medium ${tab === "mine" ? "border-purple-500 text-purple-600" : "border-transparent text-gray-500 hover:text-gray-700"}`}
            onClick={() => setTab("mine")}
          >
            我封存的{mine.length > 0 ? `（${mine.length}）` : ""}
          </button>
          <button
            className={`-mb-px border-b-2 px-4 py-2 text-sm font-medium ${tab === "received" ? "border-purple-500 text-purple-600" : "border-transparent text-gray-500 hover:text-gray-700"}`}
            onClick={() => setTab("received")}
          >
            收到的{received.length > 0 ? `（${received.length}）` : ""}
          </button>
        </div>

        {loading ? (
          <p className="py-16 text-center text-gray-400">加载中...</p>
        ) : list.length === 0 ? (
          <EmptyState
            title={tab === "mine" ? "还没有封存时光胶囊" : "还没有收到时光胶囊"}
            description={tab === "mine" ? "把此刻的心情封存给未来" : "朋友封存的胶囊会在到期后等你一起打开"}
            icon="📦"
          />
        ) : (
          <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {list.map((c) => (
              <CapsuleCard
                key={`${tab}-${c.id}`}
                capsule={c}
                now={now}
                tab={tab}
                onOpen={() => openDetail(c.id)}
                onAskDelete={() => setDeleteId(c.id)}
              />
            ))}
          </div>
        )}
      </div>

      {showCreate && (
        <CreateCapsuleModal
          currentUsername={user?.username || ""}
          onClose={() => setShowCreate(false)}
          onCreated={() => {
            setShowCreate(false);
            load();
          }}
        />
      )}

      {detailId !== null && (
        <CapsuleDetailModal
          capsule={detail}
          now={now}
          onClose={closeDetail}
          onReplied={() => onReplied(detailId)}
          onAskDelete={() => setDeleteId(detailId)}
          onAskWithdraw={() => setWithdrawId(detailId)}
        />
      )}

      <ConfirmDialog
        open={deleteId !== null}
        title="删除时光胶囊"
        description="删除后无法恢复，确定要删除吗？"
        confirmText="删除"
        danger
        onConfirm={remove}
        onCancel={() => setDeleteId(null)}
      />
      <ConfirmDialog
        open={withdrawId !== null}
        title="撤回回信"
        description="撤回后双方都无法再看到这封回信，确定撤回吗？"
        confirmText="撤回"
        danger
        onConfirm={withdraw}
        onCancel={() => setWithdrawId(null)}
      />
    </RequireAuth>
  );
}

function CapsuleCard({
  capsule,
  now,
  tab,
  onOpen,
  onAskDelete,
}: {
  capsule: Capsule;
  now: Date;
  tab: TabKey;
  onOpen: () => void;
  onAskDelete: () => void;
}) {
  const unlocked = capsule.status === "unlocked";
  return (
    <div className={`card space-y-3 ${unlocked ? "border-emerald-200" : "border-amber-100"}`}>
      <div className="flex items-center justify-between">
        <span className="text-2xl">{unlocked ? "📬" : "🔒"}</span>
        <StatusBadge status={capsule.status} kind="capsule" />
      </div>
      <div>
        <h3 className="font-semibold text-gray-800">{capsule.title}</h3>
        <p className="mt-1 text-xs text-gray-400">
          {tab === "received"
            ? `来自 ${capsule.owner_nickname}（@${capsule.owner_username}）`
            : capsule.recipient_username
              ? `共同开启：${capsule.recipient_nickname}（@${capsule.recipient_username}）`
              : "仅自己可见"}
        </p>
        <p className="mt-1 text-xs text-gray-400">解锁时间 {formatDate(capsule.unlock_at)}</p>
        {!unlocked && (
          <p className="mt-1 text-xs font-medium text-amber-600">
            ⏳ 剩余 {formatRemain(capsule.unlock_at, now)}
          </p>
        )}
      </div>
      {unlocked ? (
        <p className="line-clamp-3 text-sm text-gray-600">{capsule.content}</p>
      ) : (
        <p className="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-600">
          🔐 未到解锁时间，仅可查看标题与剩余时间
        </p>
      )}
      {unlocked && capsule.can_reply && !capsule.reply_at && (
        <p className="text-xs text-purple-500">✉️ 到期了，你可以回信一次</p>
      )}
      <div className="flex items-center justify-between">
        <button className="text-xs text-purple-500 hover:text-purple-700" onClick={onOpen}>
          {unlocked ? "查看详情" : "查看"}
        </button>
        {capsule.is_owner && (
          <button className="text-xs text-red-400 hover:text-red-600" onClick={onAskDelete}>删除</button>
        )}
      </div>
    </div>
  );
}

function CreateCapsuleModal({
  currentUsername,
  onClose,
  onCreated,
}: {
  currentUsername: string;
  onClose: () => void;
  onCreated: () => void;
}) {
  const toast = useToast();
  const [form, setForm] = useState({ title: "", content: "", recipient: "", unlock_at: "" });
  const [audioUrl, setAudioUrl] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const uploadAudio = async (file: File) => {
    try {
      const result = await uploadApi.uploadAudio(file);
      setAudioUrl(result.url);
      toast.show("音频上传成功");
    } catch (e) {
      toast.show((e as Error).message, "error");
    }
  };

  const create = async () => {
    if (!form.title || !form.content || !form.unlock_at) {
      toast.show("请填写标题、内容和解锁时间", "error");
      return;
    }
    const recipient = form.recipient.trim();
    if (recipient && recipient === currentUsername.trim()) {
      toast.show("收件人不能是自己，请填写其他朋友的用户名", "error");
      return;
    }
    setSubmitting(true);
    try {
      await capsuleApi.create({
        title: form.title,
        content: form.content,
        audio_url: audioUrl || undefined,
        unlock_at: `${form.unlock_at}:00+08:00`,
        recipient_username: recipient || undefined,
      });
      toast.show("时光胶囊已封存 📦");
      setForm({ title: "", content: "", recipient: "", unlock_at: "" });
      setAudioUrl("");
      onCreated();
    } catch (e) {
      // 账号不存在等错误：提示后留在填写页，保留已填内容
      toast.show((e as Error).message, "error");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4" onClick={onClose}>
      <div
        className="max-h-[90vh] w-full max-w-lg space-y-4 overflow-y-auto rounded-2xl bg-white p-6 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <h3 className="text-lg font-semibold text-gray-800">封存时光胶囊</h3>
        <div>
          <label className="label">标题</label>
          <input
            className="input"
            value={form.title}
            onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))}
            placeholder="给未来的信"
          />
        </div>
        <div>
          <label className="label">内容</label>
          <textarea
            className="input min-h-[100px]"
            value={form.content}
            onChange={(e) => setForm((f) => ({ ...f, content: e.target.value }))}
            placeholder="想对未来的自己说什么..."
          />
        </div>
        <div>
          <label className="label">共同开启的朋友（选填，填写用户名）</label>
          <input
            className="input"
            value={form.recipient}
            onChange={(e) => setForm((f) => ({ ...f, recipient: e.target.value }))}
            placeholder="到期后与 TA 一起打开，不能填自己"
          />
          <p className="mt-1 text-xs text-gray-400">解锁前 TA 只能看到标题和剩余时间，看不到正文、图片与语音</p>
        </div>
        <div>
          <label className="label">解锁时间</label>
          <input
            className="input"
            type="datetime-local"
            value={form.unlock_at}
            onChange={(e) => setForm((f) => ({ ...f, unlock_at: e.target.value }))}
          />
        </div>
        <div>
          <label className="label">语音留言（选填）</label>
          <input type="file" accept="audio/*" className="text-sm" onChange={(e) => e.target.files?.[0] && uploadAudio(e.target.files[0])} />
          {audioUrl && <p className="mt-1 text-xs text-emerald-600">✓ 音频已上传</p>}
        </div>
        <div className="flex justify-end gap-3">
          <button className="btn-secondary" onClick={onClose}>取消</button>
          <button className="btn-primary" disabled={submitting} onClick={create}>
            {submitting ? "封存中..." : "封存"}
          </button>
        </div>
      </div>
    </div>
  );
}

function CapsuleDetailModal({
  capsule,
  now,
  onClose,
  onReplied,
  onAskDelete,
  onAskWithdraw,
}: {
  capsule: Capsule | null;
  now: Date;
  onClose: () => void;
  onReplied: () => void;
  onAskDelete: () => void;
  onAskWithdraw: () => void;
}) {
  const toast = useToast();
  const [replyText, setReplyText] = useState("");
  const [sending, setSending] = useState(false);

  const sendReply = async () => {
    if (!capsule || !replyText.trim()) {
      toast.show("请先写下回信内容", "error");
      return;
    }
    setSending(true);
    try {
      await capsuleApi.reply(capsule.id, replyText.trim());
      toast.show("回信已发出 ✉️");
      setReplyText("");
      onReplied();
    } catch (e) {
      toast.show((e as Error).message, "error");
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4" onClick={onClose}>
      <div
        className="max-h-[90vh] w-full max-w-2xl space-y-4 overflow-y-auto rounded-2xl bg-white p-6 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        {!capsule ? (
          <p className="py-10 text-center text-gray-400">加载中...</p>
        ) : (
          <>
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="flex items-center gap-2">
                  <h3 className="text-lg font-semibold text-gray-800">{capsule.title}</h3>
                  <StatusBadge status={capsule.status} kind="capsule" />
                </div>
                <p className="mt-1 text-xs text-gray-400">
                  {capsule.is_owner
                    ? capsule.recipient_username
                      ? `共同开启：${capsule.recipient_nickname}（@${capsule.recipient_username}）`
                      : "仅自己可见"
                    : `来自 ${capsule.owner_nickname}（@${capsule.owner_username}）`}
                </p>
                <p className="mt-1 text-xs text-gray-400">解锁时间 {formatDate(capsule.unlock_at)}</p>
              </div>
              <span className="text-2xl">{capsule.status === "unlocked" ? "📬" : "🔒"}</span>
            </div>

            {capsule.status === "unlocked" ? (
              <div className="animate-unlock space-y-4">
                <p className="whitespace-pre-wrap text-sm leading-6 text-gray-700">{capsule.content}</p>
                {capsule.image_urls.length > 0 && (
                  <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
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
              </div>
            ) : (
              <div className="space-y-2 rounded-xl bg-amber-50 px-4 py-6 text-center">
                <p className="text-sm text-amber-700">🔐 胶囊尚未解锁，正文、图片与语音暂不可见</p>
                <p className="text-lg font-semibold text-amber-600">
                  ⏳ 剩余 {formatRemain(capsule.unlock_at, now)}
                </p>
              </div>
            )}

            {capsule.status === "unlocked" && (
              <ReplySection
                capsule={capsule}
                replyText={replyText}
                sending={sending}
                onReplyChange={setReplyText}
                onSend={sendReply}
                onAskWithdraw={onAskWithdraw}
              />
            )}

            <div className="flex items-center justify-between border-t border-gray-100 pt-3">
              {capsule.is_owner ? (
                <button className="text-xs text-red-400 hover:text-red-600" onClick={onAskDelete}>删除胶囊</button>
              ) : <span />}
              <button className="btn-secondary" onClick={onClose}>关闭</button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function ReplySection({
  capsule,
  replyText,
  sending,
  onReplyChange,
  onSend,
  onAskWithdraw,
}: {
  capsule: Capsule;
  replyText: string;
  sending: boolean;
  onReplyChange: (v: string) => void;
  onSend: () => void;
  onAskWithdraw: () => void;
}) {
  const hasReply = capsule.reply_at !== null;
  return (
    <div className="space-y-3 rounded-xl border border-purple-100 bg-purple-50/50 p-4">
      <h4 className="text-sm font-semibold text-purple-700">✉️ 回信</h4>
      {hasReply && !capsule.reply_withdrawn ? (
        <div className="space-y-2">
          <p className="whitespace-pre-wrap rounded-lg bg-white px-3 py-2 text-sm text-gray-700">
            {capsule.reply_content}
          </p>
          <div className="flex items-center justify-between">
            <span className="text-xs text-gray-400">回信时间 {formatDate(capsule.reply_at)}</span>
            {capsule.is_owner && (
              <button className="text-xs text-red-400 hover:text-red-600" onClick={onAskWithdraw}>撤回回信</button>
            )}
          </div>
          {!capsule.is_owner && <p className="text-xs text-gray-400">回信已发出，无法修改</p>}
        </div>
      ) : capsule.reply_withdrawn ? (
        <p className="text-xs text-gray-400">主人已撤回这封回信</p>
      ) : capsule.can_reply ? (
        <div className="space-y-2">
          <textarea
            className="input min-h-[80px]"
            value={replyText}
            onChange={(e) => onReplyChange(e.target.value)}
            placeholder="读完胶囊后，给 TA 回一封信（只能回复一次，发出后不能修改）"
          />
          <div className="flex justify-end">
            <button className="btn-primary" disabled={sending} onClick={onSend}>
              {sending ? "发送中..." : "发出回信"}
            </button>
          </div>
        </div>
      ) : (
        capsule.is_owner && <p className="text-xs text-gray-400">对方还没有回信</p>
      )}
    </div>
  );
}
