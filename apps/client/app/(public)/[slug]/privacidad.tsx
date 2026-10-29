import type { components } from '@lb/api-client';
import { businessNotice, strings } from '@lb/shared';
import { useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator } from 'react-native';
import { PrivacyNotice } from '../../../components/PrivacyNotice';
import { api } from '../../../lib/api';
import { Text } from '../../../ui';

type Notice = components['schemas']['PrivacyNotice'];

/** /<slug>/privacidad: the business's notice for its customers (public). */
export default function BusinessPrivacy() {
  const { slug } = useLocalSearchParams<{ slug: string }>();
  const [notice, setNotice] = useState<Notice | null | 'missing'>(null);

  useEffect(() => {
    api.GET('/v1/public/businesses/{slug}/privacy', { params: { path: { slug } } }).then(({ data }) => setNotice(data ?? 'missing'));
  }, [slug]);

  if (notice === 'missing') return <Text style={{ padding: 16 }}>{strings.common.notFound}</Text>;
  if (!notice) return <ActivityIndicator style={{ marginTop: 24 }} accessibilityLabel={strings.common.loading} />;
  return <PrivacyNotice title={strings.privacy.of(notice.name)} sections={businessNotice(notice)} version={notice.version} />;
}
