import { useState } from "react";
import { capsuleApi } from "@/api/capsule";
import { uploadApi } from "@/api/upload";
import { useAuth } from "@/hooks/useAuth";
import { useToast } from "@/components/Toast";

interface CapsuleCreateModalProps {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}

interface FormState {
  title: string;
  content: string;
  unlock_at: string;
  recipient_username: string;
}

const emptyForm: FormState = { title: "", content: "", unlock_at: "", recipient_username: "" };

// CapsuleCreateModal 封存胶囊：可填写收件人用户名（不能是自己；账号不存在时停留本页提示）。
export default function CapsuleCreateModal({ open, onClose, onCreated }: CapsuleCreateModalProps) {
  const toast = useToast();
  const { user } = useAuth();
  const [form, setForm] = useState<FormState>(emptyForm);
  const [imageUrls, setImageUrls] = useState<string[]>([]);
  const [audioUrl, setAudioUrl] = useState("");
  const [submitting, setSubmitting] = useState(false);

  if (!open) return null;

  const reset = () => {
    setForm(emptyForm);
    setImageUrls([]);
    setAudioUrl("");
  };

  const close = () => {
    reset();
    onClose();
  };

  const uploadImage = async (file: File) => {
    try {
      const result = await uploadApi.uploadImage(file);
      setImageUrls((urls) => [...urls, result.url]);
      toast.show("图片上传成功");
    } catch (e) {
      toast.show((e as Error).message, "error");
    }
  };

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
    // 不能选自己：前端先拦一次，后端仍会权威校验
    if (form.recipient_username.trim() && user && form.recipient_username.trim() === user.username) {
      toast.show("收件人不能填写自己的用户名", "error");
      return;
    }
    setSubmitting(true);
    try {
      await capsuleApi.create({
        title: form.title,
        content: form.content,
        image_urls: imageUrls,
        audio_url: audioUrl || undefined,
        unlock_at: `${form.unlock_at}:00+08:00`,
        recipient_username: form.recipient_username.trim() || undefined,
      });
      toast.show("时光胶囊已封存 📦");
      reset();
      onCreated();
      onClose();
    } catch (e) {
      // 账号不存在 / 不能是自己：提示后留在填写页，表单内容保留
      toast.show((e as Error).message, "error");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4" onClick={close}>
      <div className="max-h-[85vh] w-full max-w-lg space-y-4 overflow-y-auto rounded-2xl bg-white p-6 shadow-xl" onClick={(e) => e.stopPropagation()}>
        <h3 className="text-lg font-semibold text-gray-800">封存时光胶囊</h3>
        <div>
          <label className="label">标题</label>
          <input className="input" value={form.title} onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))} placeholder="给未来的信" />
        </div>
        <div>
          <label className="label">收件人用户名（选填）</label>
          <input
            className="input"
            value={form.recipient_username}
            onChange={(e) => setForm((f) => ({ ...f, recipient_username: e.target.value }))}
            placeholder="填写朋友的用户名，到期后一起打开；不能填自己"
          />
          <p className="mt-1 text-xs text-gray-400">留空则仅自己可见；账号不存在时会提示并停留本页</p>
        </div>
        <div>
          <label className="label">内容</label>
          <textarea className="input min-h-[100px]" value={form.content} onChange={(e) => setForm((f) => ({ ...f, content: e.target.value }))} placeholder="想对未来的你们说什么..." />
        </div>
        <div>
          <label className="label">图片（选填，可多张）</label>
          <input type="file" accept="image/*" className="text-sm" onChange={(e) => e.target.files?.[0] && uploadImage(e.target.files[0])} />
          {imageUrls.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-2">
              {imageUrls.map((url) => (
                // eslint-disable-next-line @next/next/no-img-element
                <img key={url} src={url} alt="已上传" className="h-14 w-14 rounded-lg object-cover" />
              ))}
            </div>
          )}
        </div>
        <div>
          <label className="label">解锁时间</label>
          <input className="input" type="datetime-local" value={form.unlock_at} onChange={(e) => setForm((f) => ({ ...f, unlock_at: e.target.value }))} />
        </div>
        <div>
          <label className="label">语音留言（选填）</label>
          <input type="file" accept="audio/*" className="text-sm" onChange={(e) => e.target.files?.[0] && uploadAudio(e.target.files[0])} />
          {audioUrl && <p className="mt-1 text-xs text-emerald-600">✓ 音频已上传</p>}
        </div>
        <div className="flex justify-end gap-3">
          <button className="btn-secondary" onClick={close}>取消</button>
          <button className="btn-primary" disabled={submitting} onClick={create}>{submitting ? "封存中..." : "封存"}</button>
        </div>
      </div>
    </div>
  );
}
