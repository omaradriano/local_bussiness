import { strings } from '@lb/shared';
import { useState, type ReactNode } from 'react';
import { Modal, Pressable, View } from 'react-native';
import { Button, Field, Text, radius, useUI } from '../ui';

const norm = (s: string) => s.trim().split(/\s+/).join(' ').toLowerCase();

/**
 * Confirmation for a destructive action (in-app, not the browser's confirm). With `typeToConfirm`
 * the button stays off until that text is typed (spaces and case don't matter, like the API).
 * `primary` is for a confirmation that isn't destructive; `children` go under the body (a choice to make).
 */
export function ConfirmDialog({ open, title, body, confirmLabel, typeToConfirm, typeLabel, busy, primary, children, onConfirm, onClose }: {
  open: boolean; title: string; body: string; confirmLabel: string; typeToConfirm?: string; typeLabel?: string;
  busy?: boolean; primary?: boolean; children?: ReactNode; onConfirm: () => void; onClose: () => void;
}) {
  const ui = useUI();
  const [typed, setTyped] = useState('');
  const ready = !typeToConfirm || norm(typed) === norm(typeToConfirm);
  const close = () => { setTyped(''); onClose(); };

  return (
    <Modal visible={open} transparent animationType="fade" onRequestClose={close}>
      <Pressable accessibilityLabel={strings.photos.close} onPress={close}
        style={{ flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', justifyContent: 'center', alignItems: 'center', padding: 16 }}>
        {/* Stop taps inside the panel from closing it. */}
        <Pressable onPress={() => {}} style={{ width: '100%', maxWidth: 420, backgroundColor: ui.surface, borderRadius: radius.lg, padding: 20, gap: 12 }}>
          <Text variant="title" accessibilityRole="header">{title}</Text>
          <Text color="labelSecondary">{body}</Text>
          {children}
          {typeToConfirm && (
            <Field label={typeLabel ?? typeToConfirm} value={typed} onChangeText={setTyped} autoCapitalize="none" autoCorrect={false} />
          )}
          <View style={{ gap: 8 }}>
            <Button variant={primary ? 'primary' : 'destructive'} busy={busy} disabled={!ready} onPress={onConfirm}>{confirmLabel}</Button>
            <Button variant="plain" disabled={busy} onPress={close}>{strings.confirm.cancel}</Button>
          </View>
        </Pressable>
      </Pressable>
    </Modal>
  );
}
