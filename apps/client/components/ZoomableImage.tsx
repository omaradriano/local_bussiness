import { strings } from '@lb/shared';
import { Image, type ImageProps } from 'expo-image';
import { useState } from 'react';
import { Modal, Pressable, Text, View } from 'react-native';

/** A thumbnail that opens the image full screen on tap (fullSource if given, e.g. a bigger CDN variant). */
export function ZoomableImage({ fullSource, ...props }: ImageProps & { fullSource?: string }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Pressable accessibilityRole="imagebutton" accessibilityLabel={`${props.accessibilityLabel ?? ''} · ${strings.photos.enlarge}`} onPress={() => setOpen(true)}>
        <Image {...props} />
      </Pressable>
      <Modal visible={open} transparent animationType="fade" onRequestClose={() => setOpen(false)}>
        <Pressable accessibilityRole="button" accessibilityLabel={strings.photos.close} onPress={() => setOpen(false)}
          style={{ flex: 1, backgroundColor: 'rgba(0,0,0,0.9)', padding: 16, gap: 8 }}>
          <View style={{ alignItems: 'flex-end' }}><Text style={{ color: 'white', fontSize: 16 }}>✕ {strings.photos.close}</Text></View>
          <Image source={fullSource ?? props.source} placeholder={props.placeholder} style={{ flex: 1, width: '100%' }} contentFit="contain"
            accessibilityLabel={props.accessibilityLabel} />
        </Pressable>
      </Modal>
    </>
  );
}
