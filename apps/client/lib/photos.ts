import { fitWithin, strings } from '@lb/shared';
import { ImageManipulator, SaveFormat } from 'expo-image-manipulator';
import * as ImagePicker from 'expo-image-picker';
import { api } from './api';
import { errorText, fieldErrors } from './errors';

/**
 * Pick a photo, shrink it to 2048 px (JPEG, also applies the phone's EXIF rotation),
 * upload it straight to S3 with the presigned URL and confirm it. Returns an error message or null.
 */
export async function pickAndUploadPhoto(businessId: string, itemId: string): Promise<string | null | 'cancelled'> {
  const perm = await ImagePicker.requestMediaLibraryPermissionsAsync();
  if (!perm.granted) return strings.photos.permission;

  const picked = await ImagePicker.launchImageLibraryAsync({ mediaTypes: 'images', quality: 1 });
  if (picked.canceled || !picked.assets[0]) return 'cancelled';
  const asset = picked.assets[0];

  const ctx = ImageManipulator.manipulate(asset.uri);
  const target = fitWithin(asset.width, asset.height);
  if (target) ctx.resize(target);
  const rendered = await ctx.renderAsync();
  const saved = await rendered.saveAsync({ format: SaveFormat.JPEG, compress: 0.85 });
  const blob = await (await fetch(saved.uri)).blob();

  const path = { businessId, itemId };
  const created = await api.POST('/v1/b/{businessId}/catalog/{itemId}/images/', {
    params: { path },
    body: { content_type: 'image/jpeg', size: blob.size },
  });
  if (!created.data) return fieldErrors(created.error).file ?? errorText(created.error, created.response.status);

  const put = await fetch(created.data.upload_url, { method: 'PUT', headers: created.data.upload_headers, body: blob });
  if (!put.ok) return `${strings.common.error} (S3 ${put.status})`;

  const done = await api.POST('/v1/b/{businessId}/catalog/{itemId}/images/{imageId}/complete', {
    params: { path: { ...path, imageId: created.data.image.id } },
  });
  return done.response.ok ? null : fieldErrors(done.error).file ?? errorText(done.error, done.response.status);
}
