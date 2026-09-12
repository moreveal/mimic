"""Regression for partial-interface and mixin provenance, independent of Chrome."""
import unittest
import json
from generate_compat import normalize_idl, extended, surface_catalog

class ProvenanceTests(unittest.TestCase):
    def test_retained_conditional_exposure_preserves_statics(self):
        catalog={'declarations':[{'kind':'interface','name':'Example','extended':{'Exposed(Window Feature, Worker WorkerFeature)':True},'members':[{'kind':'operation','name':'availability','static':True}]}]}
        projected=json.loads(surface_catalog(catalog))[0]
        self.assertEqual(projected['exposed'],['Window','Worker'])
        self.assertTrue(projected['members'][0]['static'])
        self.assertNotIn('Exposed',catalog['declarations'][0]['extended'])
    def test_namespace_constants_survive_projection(self):
        catalog=normalize_idl({'flags.idl':'[Exposed=(Window, Worker), SecureContext] namespace ExampleFlags { const unsigned long READ = 0x0001; const unsigned long WRITE = 2; };'}, {'chrome_version':'test','chromium_ref':'pinned'})
        projected=json.loads(surface_catalog(catalog))
        self.assertEqual(len(projected),1)
        self.assertEqual(projected[0]['kind'],'namespace')
        self.assertEqual(projected[0]['exposed'],['Window','Worker'])
        self.assertEqual([(m['name'],m['value']) for m in projected[0]['members']],[('READ','0x0001'),('WRITE','2')])

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
