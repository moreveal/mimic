"""Regression for partial-interface and mixin provenance, independent of Chrome."""
import unittest
from generate_compat import normalize_idl, extended

class ProvenanceTests(unittest.TestCase):
    def test_conditional_exposure(self):
        self.assertEqual(extended('[Exposed(Window WebHID, DedicatedWorker WorkerHID), SecureContext]'),
                         {'Exposed':['Window','DedicatedWorker'],'ExposureFeatures':{'Window':'WebHID','DedicatedWorker':'WorkerHID'},'SecureContext':True})
    def test_conditions_stay_with_declaring_source(self):
        catalog=normalize_idl({
            'base.idl':'[Exposed=Window] interface Navigator { readonly attribute DOMString vendorSub; };',
            'partial.idl':'[SecureContext, RuntimeEnabled=DeviceFlag] partial interface Navigator { readonly attribute Device device; };',
            'mixin.idl':'[SecureContext] interface mixin NavigatorStorage { readonly attribute StorageManager storage; }; Navigator includes NavigatorStorage;',
        },{'chrome_version':'test','chromium_ref':'pinned'})
        nav=next(d for d in catalog['declarations'] if d['name']=='Navigator')
        self.assertEqual(nav['extended'],{'Exposed':'Window'})
        members={m['name']:m for m in nav['members']}
        self.assertEqual(members['device']['origin']['source'],'partial.idl')
        self.assertEqual(members['device']['origin']['extended']['RuntimeEnabled'],'DeviceFlag')
        self.assertTrue(members['storage']['origin']['extended']['SecureContext'])
        self.assertEqual(members['storage']['origin']['interface'],'NavigatorStorage')
        self.assertNotIn('SecureContext',members['vendorSub']['origin']['extended'])

if __name__=='__main__':unittest.main()
