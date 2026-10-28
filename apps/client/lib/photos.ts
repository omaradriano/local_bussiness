import { fitWithin, strings } from '@lb/shared';
import { ImageManipulator, SaveFormat } from 'expo-image-manipulator';
import * as ImagePicker from 'expo-image-picker';
import { api } from './api';
import { errorText, fieldErrors } from './errors';

/** A photo chosen and already shrunk on the device, not uploaded yet. */
export type PickedPhoto = { uri: string };

/** Shrink to 2048 px as JPEG (also applies the phone's EXIF rotation). */
async function shrink(asset: ImagePicker.ImagePickerAsset): Promise<PickedPhoto> {
  const ctx = ImageManipulator.manipulate(asset.uri);
  const target = fitWithin(asset.width, asset.height);
  if (target) ctx.resize(target);
  const rendered = await ctx.renderAsync();
  const saved = await rendered.saveAsync({ format: SaveFormat.JPEG, compress: 0.85 });
  return { uri: saved.uri };
}

/**
 * Pick up to `limit` photos at once and shrink each one. Returns the photos and how many were left out
 * because of the limit (web can't cap the selection), 'cancelled', or an error message.
 */
export async function pickPhotos(limit: number): Promise<{ photos: PickedPhoto[]; dropped: number } | 'cancelled' | string> {
  const perm = await ImagePicker.requestMediaLibraryPermissionsAsync();
  if (!perm.granted) return strings.photos.permission;

  const picked = await ImagePicker.launchImageLibraryAsync({
    mediaTypes: 'images',
    quality: 1,
    allowsMultipleSelection: true,
    selectionLimit: limit,
    orderedSelection: true,
  });
  if (picked.canceled || !picked.assets.length) return 'cancelled';
  const chosen = picked.assets.slice(0, limit);
  const photos: PickedPhoto[] = [];
  for (const asset of chosen) photos.push(await shrink(asset)); // one at a time: keeps memory low on phones
  return { photos, dropped: picked.assets.length - chosen.length };
}

/** Upload a picked photo to an existing item: presigned PUT straight to S3, then confirm. Error message or null. */
export async function uploadPhoto(businessId: string, itemId: string, photo: PickedPhoto): Promise<string | null> {
  const blob = await (await fetch(photo.uri)).blob();
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

/** Upload several photos in order; returns the first error, if any. */
export async function uploadPhotos(businessId: string, itemId: string, photos: PickedPhoto[]): Promise<string | null> {
  for (const p of photos) {
    const err = await uploadPhoto(businessId, itemId, p);
    if (err) return err;
  }
  return null;
}

/** Upload a picked photo for a booking step to its private bucket; returns the key or an error message. */
export async function uploadStepPhoto(businessId: string, bookingId: string, stepId: string, photo: PickedPhoto): Promise<{ key: string } | { error: string }> {
  const blob = await (await fetch(photo.uri)).blob();
  const created = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/uploads', {
    params: { path: { businessId, bookingId } },
    body: { step_id: stepId, content_type: 'image/jpeg', size: blob.size },
  });
  if (!created.data) return { error: fieldErrors(created.error).file ?? errorText(created.error, created.response.status) };
  const put = await fetch(created.data.upload_url, { method: 'PUT', headers: created.data.upload_headers, body: blob });
  if (!put.ok) return { error: `${strings.common.error} (S3 ${put.status})` };
  return { key: created.data.key };
}

/** Upload a picked photo of a customer document to the private bucket; returns the key or an error message. */
export async function uploadCustomerDoc(businessId: string, customerId: string, docType: 'ine' | 'proof_of_address' | 'guarantor_id', photo: PickedPhoto): Promise<{ key: string } | { error: string }> {
  const blob = await (await fetch(photo.uri)).blob();
  const created = await api.POST('/v1/b/{businessId}/customers/{customerId}/uploads', {
    params: { path: { businessId, customerId } },
    body: { doc_type: docType, content_type: 'image/jpeg', size: blob.size },
  });
  if (!created.data) return { error: fieldErrors(created.error).file ?? errorText(created.error, created.response.status) };
  const put = await fetch(created.data.upload_url, { method: 'PUT', headers: created.data.upload_headers, body: blob });
  if (!put.ok) return { error: `${strings.common.error} (S3 ${put.status})` };
  return { key: created.data.key };
}

/** Upload a photo that proves a payment (transfer screenshot, card ticket); returns the key or an error message. */
export async function uploadPaymentEvidence(businessId: string, bookingId: string, photo: PickedPhoto): Promise<{ key: string } | { error: string }> {
  const blob = await (await fetch(photo.uri)).blob();
  const created = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/uploads', {
    params: { path: { businessId, bookingId } },
    body: { purpose: 'payment', content_type: 'image/jpeg', size: blob.size },
  });
  if (!created.data) return { error: fieldErrors(created.error).file ?? errorText(created.error, created.response.status) };
  const put = await fetch(created.data.upload_url, { method: 'PUT', headers: created.data.upload_headers, body: blob });
  if (!put.ok) return { error: `${strings.common.error} (S3 ${put.status})` };
  return { key: created.data.key };
}
