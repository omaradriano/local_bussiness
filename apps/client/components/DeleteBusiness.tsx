import { strings } from '@lb/shared';
import { useState } from 'react';
import { api } from '../lib/api';
import { errorText } from '../lib/errors';
import { Cell, Group, useToast } from '../ui';
import { ConfirmDialog } from './ConfirmDialog';

const t = strings.deleteBusiness;

/**
 * «Zona de peligro»: delete the business with everything in it, typing its name to confirm. The owner
 * uses their business route; the platform admin, the admin one. onDeleted moves away from it.
 */
export function DeleteBusiness({ businessId, name, asAdmin, onDeleted }: {
  businessId: string; name: string; asAdmin?: boolean; onDeleted: () => void;
}) {
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  async function remove() {
    setBusy(true);
    const params = { path: { businessId } };
    const body = { confirm: name };
    const res = asAdmin
      ? await api.DELETE('/v1/admin/businesses/{businessId}', { params, body })
      : await api.DELETE('/v1/b/{businessId}/', { params, body });
    setBusy(false);
    if (!res.response.ok) return toast(errorText(res.error, res.response.status), 'error');
    setOpen(false);
    toast(t.done);
    onDeleted();
  }

  return (
    <Group label={t.title} footer={t.hint}>
      <Cell icon="trash" label={t.action} destructive onPress={() => setOpen(true)} />
      <ConfirmDialog open={open} title={t.confirmTitle(name)} body={t.body} confirmLabel={t.confirm}
        typeToConfirm={name} typeLabel={t.typeName(name)} busy={busy} onConfirm={remove} onClose={() => setOpen(false)} />
    </Group>
  );
}
