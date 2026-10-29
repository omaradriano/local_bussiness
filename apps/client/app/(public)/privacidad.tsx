import { platformNotice, PRIVACY_VERSION, strings } from '@lb/shared';
import { PrivacyNotice } from '../../components/PrivacyNotice';

/** /privacidad: the platform's notice (owners and staff). */
export default function PlatformPrivacy() {
  return <PrivacyNotice title={strings.privacy.title} sections={platformNotice()} version={PRIVACY_VERSION} />;
}
