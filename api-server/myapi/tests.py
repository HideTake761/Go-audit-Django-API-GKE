from rest_framework import status
from rest_framework.test import APITestCase
from django.urls import reverse
from myapi.models import Item

class ItemAPITestCase(APITestCase):

    def setUp(self):
        # テスト用のデータを2件作成
        self.item1 = Item.objects.create(product='Apple', price=100)
        self.item2 = Item.objects.create(product='Banana', price=50)
        self.list_url = reverse('item-list')
        self.detail_url = lambda pk: reverse('item-detail', kwargs={'pk': pk})
        # reverse():urls.pyで設定した名前付きURLパターンをもとにURLを生成するためのメソッド
        # /myapi/urls.pyのrouter.register(r'items', ItemViewSet)でルーターが/itemsという
        # パスを生成するよう設定。
        # 例:items/ (商品の一覧取得・作成用)、items/<int:pk>/ (特定の商品取得・更新・削除用)

    def test_create_item(self):
        data = {'product': 'Orange', 'price': 120}
        response = self.client.post(self.list_url, data, format='json')
        self.assertEqual(response.status_code, status.HTTP_201_CREATED)
        self.assertEqual(Item.objects.count(), 3)

    def test_list_items(self):
        response = self.client.get(self.list_url)
        self.assertEqual(response.status_code, status.HTTP_200_OK)
        self.assertEqual(len(response.data), 2)

    def test_get_item_detail(self):
        response = self.client.get(self.detail_url(self.item1.id))
        self.assertEqual(response.status_code, status.HTTP_200_OK)
        self.assertEqual(response.data['product'], self.item1.product)

    def test_update_item(self):
        data = {'product': 'Updated Apple', 'price': 150}
        response = self.client.put(self.detail_url(self.item1.id), data, format='json')
        self.assertEqual(response.status_code, status.HTTP_200_OK)
        self.item1.refresh_from_db()
        self.assertEqual(self.item1.product, 'Updated Apple')
        self.assertEqual(self.item1.price, 150)

    def test_partial_update_item(self):
        data = {'price': 180}
        response = self.client.patch(self.detail_url(self.item1.id), data, format='json')
        self.assertEqual(response.status_code, status.HTTP_200_OK)
        self.item1.refresh_from_db()
        self.assertEqual(self.item1.price, 180)

    def test_delete_item(self):
        response = self.client.delete(self.detail_url(self.item2.id))
        self.assertEqual(response.status_code, status.HTTP_204_NO_CONTENT)
        self.assertFalse(Item.objects.filter(id=self.item2.id).exists())

    def test_create_invalid_item(self):
        data = {'product': '', 'price': -50} # 無効なデータ（空の名前、マイナス価格）
        response = self.client.post(self.list_url, data, format='json')
        self.assertEqual(response.status_code, status.HTTP_400_BAD_REQUEST)

    def test_get_nonexistent_item(self):
        response = self.client.get(self.detail_url(9999)) # 存在しないID
        self.assertEqual(response.status_code, status.HTTP_404_NOT_FOUND)
