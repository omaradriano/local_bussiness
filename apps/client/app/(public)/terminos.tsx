import { strings, termsSections, TERMS_VERSION } from '@lb/shared';
import { PrivacyNotice } from '../../components/PrivacyNotice';

/** /terminos: terms and conditions of the platform. */
export default function Terms() {
  return <PrivacyNotice title={strings.privacy.terms} headerTitle={strings.privacy.terms} sections={termsSections()} version={TERMS_VERSION} />;
}
