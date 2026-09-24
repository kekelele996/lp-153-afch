import { useCallback, useEffect, useState } from "react";
import type { Capsule } from "@/api/capsule";
import { capsuleApi } from "@/api/capsule";
import CapsuleCard from "@/components/CapsuleCard";
import CapsuleCreateModal from "@/components/CapsuleCreateModal";
import CapsuleDetailModal from "@/components/CapsuleDetailModal";
import ConfirmDialog from "@/components/ConfirmDialog";
import EmptyState from "@/components/EmptyState";
import RequireAuth from "@/components/RequireAuth";
import { useToast } from "@/components/Toast";
import { useAuth } from "@/hooks/useAuth";

type Tab = "mine" | "received";

export default function Capsules() {
  const { isAuthed } = useAuth();
  const toast = useToast();
  const [tab, setTab] = useState<Tab>("mine");
  const [mine, setMine] = useState<Capsule[]>([]);
  const [received, setReceived] = useState<Capsule[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [detailId, setDetailId] = useState<number | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Capsule | null>(null);

  const loadMine = useCallback(async () => {
    const data = await capsuleApi.mine({ page: 1, page_size: 50 });
    setMine(data.items);
  }, []);

  const loadReceived = useCallback(async () => {
    const data = await capsuleApi.received({ page: 1, page_size: 50 });
    setReceived(data.items);
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      await Promise.all([loadMine(), loadReceived()]);
    } catch (e) {
      toast.show((e as Error).message, "error");
    } finally {
      setLoading(false);
    }
  }, [loadMine, loadReceived, toast]);

  useEffect(() => {
    if (isAuthed()) load();
  }, [isAuthed, load]);

  const remove = async () => {
    if (!deleteTarget) return;
    try {
      await capsuleApi.remove(deleteTarget.id);
      toast.show("胶囊已删除");
      setDeleteTarget(null);
      loadMine();
    } catch (e) {
      toast.show((e as Error).message, "error");
    }
  };

  const capsules = tab === "mine" ? mine : received;

  return (
    <RequireAuth>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-purple-700">📦 时光胶囊</h1>
            <p className="mt-1 text-sm text-gray-500">写给未来的自己与朋友，到时间才能一起打开</p>
          </div>
          <button className="btn-primary" onClick={() => setShowCreate(true)}>封存新胶囊</button>
        </div>

        <div className="flex gap-2 border-b border-gray-100">
          <button
            className={`-mb-px border-b-2 px-4 py-2 text-sm font-medium transition ${
              tab === "mine" ? "border-purple-500 text-purple-600" : "border-transparent text-gray-400 hover:text-gray-600"
            }`}
            onClick={() => setTab("mine")}
          >
            我封存的{mine.length > 0 ? ` (${mine.length})` : ""}
          </button>
          <button
            className={`-mb-px border-b-2 px-4 py-2 text-sm font-medium transition ${
              tab === "received" ? "border-purple-500 text-purple-600" : "border-transparent text-gray-400 hover:text-gray-600"
            }`}
            onClick={() => setTab("received")}
          >
            收到的{received.length > 0 ? ` (${received.length})` : ""}
          </button>
        </div>

        {loading ? (
          <p className="py-16 text-center text-gray-400">加载中...</p>
        ) : capsules.length === 0 ? (
          <EmptyState
            title={tab === "mine" ? "还没有封存的时光胶囊" : "还没有收到时光胶囊"}
            description={tab === "mine" ? "把此刻的心情封存给未来，还可以指定一位朋友共同开启" : "当朋友封存胶囊时填上你的用户名，这里就会出现"}
            icon="📦"
          />
        ) : (
          <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {capsules.map((c) => (
              <CapsuleCard
                key={c.id}
                capsule={c}
                onOpen={(item) => setDetailId(item.id)}
                onDelete={tab === "mine" ? setDeleteTarget : undefined}
                onReload={tab === "mine" ? loadMine : loadReceived}
              />
            ))}
          </div>
        )}
      </div>

      <CapsuleCreateModal open={showCreate} onClose={() => setShowCreate(false)} onCreated={loadMine} />
      <CapsuleDetailModal capsuleId={detailId} onClose={() => setDetailId(null)} onChanged={load} />

      <ConfirmDialog
        open={deleteTarget !== null}
        title="删除时光胶囊"
        description={deleteTarget?.recipient_username ? "删除后收件人也将无法看到这封胶囊与回信，确定要删除吗？" : "删除后无法恢复，确定要删除吗？"}
        confirmText="删除"
        danger
        onConfirm={remove}
        onCancel={() => setDeleteTarget(null)}
      />
    </RequireAuth>
  );
}
