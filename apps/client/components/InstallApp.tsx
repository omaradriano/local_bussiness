import { strings } from '@lb/shared';
import { useState, type ReactNode } from 'react';
import { Modal, Pressable, ScrollView, View } from 'react-native';
import { useInstall } from '../lib/install';
import { Button, Step, Text, radius, useToast, useUI } from '../ui';

const t = strings.nav;

/**
 * "Instalar app" of the menu: the browser's dialog where there is one, the Share-menu steps on
 * iPhone/iPad. `show` is false where it doesn't apply (installed, native, unsupported browser).
 */
export function useInstallAction(): { show: boolean; label: string; onPress: () => void; sheet: ReactNode } {
  const { mode, prompt } = useInstall();
  const toast = useToast();
  const [steps, setSteps] = useState(false);
  const onPress = mode === 'ios' ? () => setSteps(true) : () => { prompt().then((ok) => ok && toast(t.installed)); };
  return { show: mode !== null, label: t.install, onPress, sheet: <IosSteps open={steps} onClose={() => setSteps(false)} /> };
}

function IosSteps({ open, onClose }: { open: boolean; onClose: () => void }) {
  const ui = useUI();
  const s = t.installIos;
  return (
    <Modal visible={open} transparent animationType="fade" onRequestClose={onClose}>
      <Pressable accessibilityLabel={strings.photos.close} onPress={onClose}
        style={{ flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', justifyContent: 'center', alignItems: 'center', padding: 16 }}>
        {/* Stop taps inside the panel from closing it. */}
        <Pressable onPress={() => {}} style={{ width: '100%', maxWidth: 400, maxHeight: '90%', backgroundColor: ui.bg, borderRadius: radius.lg, padding: 16 }}>
          <ScrollView contentContainerStyle={{ gap: 12 }}>
            <Text variant="title" accessibilityRole="header">{s.title}</Text>
            {s.steps.map((st, i) => <Step key={st.title} n={i + 1} title={st.title} description={st.description} />)}
            <View style={{ marginTop: 4 }}>
              <Button variant="primary" onPress={onClose}>{s.done}</Button>
            </View>
          </ScrollView>
        </Pressable>
      </Pressable>
    </Modal>
  );
}
