from locust import HttpUser, between, task


class WebsiteUser(HttpUser):
    wait_time = between(5, 15)
    
    @task
    def index(self):
        self.client.get("/")

    @task
    def get_cart(self):
        self.client.get("/carts/1")

    @task
    def get_cart_items(self):
        self.client.get("/carts/1/items")

    @task
    def add_item_to_cart(self):
        ITEM_ID="03fef6ac-1896-4ce8-bd69-b798f85c6e0b"
        CARTS_ID="3395a43e-2d88-40de-b95f-e00e1502085b"
        json_payload = "{\"id\":\"" + CARTS_ID + "\", \"itemId\":\"" + ITEM_ID + "\", \"price\":\"99.90\"}"
        self.client.post("/carts/1/items", json_payload)

    @task
    def health(self):
        self.client.get("/health")

