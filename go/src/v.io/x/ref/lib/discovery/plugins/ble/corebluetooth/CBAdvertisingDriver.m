// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#import "CBAdvertisingDriver.h"
#import "CBDriver.h"
#import "CBLog.h"
#import "CBUtil.h"

static const void *kRotateAdDelay = &kRotateAdDelay;

@interface CBAdvertisingDriver ()
@property(nonatomic, assign) AdvertisingState advertisingState;
@property(nonatomic, assign) BOOL rotateAd;
@property(nonatomic, strong) NSDate *requestedAdvertisingStart;
/** advertisedServices is the same as the keys of services, but the ordering maintained here is
 how we do this rotation business. */
@property(nonatomic, strong) NSMutableArray<CBUUID *> *_Nonnull advertisedServices;
/** All added services by uuid */
@property(nonatomic, strong) NSMutableDictionary<CBUUID *, CBMutableService *> *_Nonnull services;
@property(nonatomic, strong)
    NSMutableDictionary<CBUUID *, CBAddServiceHandler> *_Nonnull addServiceHandlers;
@property(nonatomic, strong) NSMutableDictionary<CBUUID *, NSDictionary<CBUUID *, NSData *> *>
    *_Nonnull serviceCharacteristics;
@end

@implementation CBAdvertisingDriver

- (id _Nullable)initWithQueue:(dispatch_queue_t _Nonnull)queue {
  if (self = [super init]) {
    self.queue = queue;
    self.peripheral =
        [[CBPeripheralManager alloc] initWithDelegate:self
                                                queue:self.queue
                                              options:@{
#if TARGET_OS_IPHONE
                                                CBPeripheralManagerOptionShowPowerAlertKey : @NO
#endif
                                              }];
    self.services = [NSMutableDictionary new];
    self.serviceCharacteristics = [NSMutableDictionary new];
    self.addServiceHandlers = [NSMutableDictionary new];
    self.advertisedServices = [NSMutableArray new];
    self.advertisingState = AdvertisingStateNotAdvertising;
    self.rotateAdDelay = 1;
  }
  return self;
}

- (void)dealloc {
  CBDispatchSync(self.queue, ^{
    self.peripheral.delegate = nil;
    if (self.isHardwarePoweredOn) {
      [self.peripheral stopAdvertising];
      [self.peripheral removeAllServices];
    }
  });
}

- (void)addService:(CBUUID *_Nonnull)uuid
   characteristics:(NSDictionary<CBUUID *, NSData *> *_Nonnull)characteristics
          callback:(CBAddServiceHandler _Nonnull)handler {
  // Allow shutdown in between to occur.
  __weak typeof(self) this = self;
  dispatch_async(self.queue, ^{
    if (!this) {
      return;
    }
    if (this.services[uuid]) {
      handler(uuid, [NSError errorWithDomain:kCBDriverErrorDomain
                                        code:CBDriverErrorServiceAlreadyAdded
                                    userInfo:@{
                                      NSLocalizedDescriptionKey : @"Service already added"
                                    }]);
      return;
    }
    CBMutableService *service =
        [CBMutableService cb_mutableService:uuid withReadOnlyCharacteristics:characteristics];
    // Save the service & characteristics for later reads
    this.services[uuid] = service;
    this.serviceCharacteristics[uuid] = characteristics;
    // Insert this as the next service advertised
    [this.advertisedServices insertObject:uuid atIndex:0];
    CBDebugLog(@"Advertised services is now %@", this.advertisedServices);
    // We can't do anything if we're not powered on -- but it's queued to be added whenever it is
    if (this.isHardwarePoweredOn) {
      this.addServiceHandlers[uuid] = handler;
      [this.peripheral addService:service];
      CBInfoLog(@"Added service %@", service);
      [this stopAdvertising];
      [this startAdvertising];
    } else {
      // Tell handler it was a success since we can't know any true error until the hardware is
      // ready.
      CBInfoLog(@"Queued service %@", service);
      handler(service.UUID, nil);
    }
  });
}

- (NSUInteger)serviceCount {
  return self.services.count;
}

- (void)removeService:(CBUUID *_Nonnull)uuid {
  CBInfoLog(@"removeService: %@", uuid.UUIDString);
  CBDispatchSync(self.queue, ^{
    CBMutableService *service = self.services[uuid];
    if (!service) {
      return;
    }
    if (self.isHardwarePoweredOn) {
      [self.peripheral removeService:service];
    }
    self.services[uuid] = nil;
    self.serviceCharacteristics[uuid] = nil;
    [self.advertisedServices removeObject:uuid];
    self.addServiceHandlers[uuid] = nil;
    // Undo the service from our current ads
    [self stopAdvertising];
    // startAdvertising will check the current count before enabling
    [self startAdvertising];
  });
}

- (void)addAllServices {
  [self threadSafetyCheck];
  if (!self.isHardwarePoweredOn) {
    CBErrorLog(@"Can't add all services -- BLE hardware state isn't powered on");
    return;
  }
  [self.peripheral removeAllServices];
  for (CBMutableService *service in self.services.allValues) {
    [self.peripheral addService:service];
  }
}

- (void)startAdvertising {
  [self threadSafetyCheck];
  if (self.advertisingState != AdvertisingStateNotAdvertising) {
    CBErrorLog(@"Not starting advertising when in state %d", self.advertisingState);
    return;
  }
  if (!self.advertisedServices.count) {
    CBDebugLog(@"Nothing to advertise");
    return;
  }
  if (!self.isHardwarePoweredOn) {
    CBInfoLog(@"Not powered on -- can't advertise yet");
    return;
  }
  NSDictionary *ad = @{CBAdvertisementDataServiceUUIDsKey : self.advertisedServices};
  self.advertisingState = AdvertisingStateStarting;
  CBDebugLog(@"startAdvertising %@", self.advertisedServices);
  [self invalidateRotateAdTimer];
  // When we get the callback advertising started then we reschedule rotation.
  self.requestedAdvertisingStart = [NSDate date];
  [self.peripheral startAdvertising:ad];
}

- (void)stopAdvertising {
  [self threadSafetyCheck];
  CBDebugLog(@"stopAdvertising");
  self.advertisingState = AdvertisingStateNotAdvertising;
  if (self.isHardwarePoweredOn) {
    [self.peripheral stopAdvertising];
  }
  [self invalidateRotateAdTimer];
}

- (void)scheduleRotateAdTimer:(NSTimeInterval)delay {
  self.rotateAd = YES;
  __weak typeof(self) this = self;
  dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(delay * NSEC_PER_SEC)), self.queue, ^{
    [this rotateAdTimerDidFire];
  });
}

- (void)invalidateRotateAdTimer {
  self.rotateAd = NO;
}

- (void)rotateAdTimerDidFire {
  [self threadSafetyCheck];
  if (!self.rotateAd) return;
  switch (self.advertisingState) {
    case AdvertisingStateNotAdvertising:
      CBDebugLog(@"Not advertising - expiring timer");
      [self invalidateRotateAdTimer];
      return;
    case AdvertisingStateStarting:
      CBDebugLog(@"Advertising still starting, ignoring timer");
      return;
    case AdvertisingStateAdvertising:
      if (self.advertisedServices.count < 2) {
        CBDebugLog(@"Not enough services to rotate bluetooth ad - expiring timer");
        [self invalidateRotateAdTimer];
        return;
      }
      break;
  }

  // Rotate services
  CBUUID *head = [self.advertisedServices firstObject];
  [self.advertisedServices removeObjectAtIndex:0];
  [self.advertisedServices addObject:head];
  [self stopAdvertising];
  [self startAdvertising];  // This will reschedule the next timer if needed
}

- (void)peripheralManagerDidUpdateState:(CBPeripheralManager *)peripheral {
  switch (peripheral.state) {
    case CBPeripheralManagerStatePoweredOn:
      CBInfoLog(@"CBPeripheralManagerStatePoweredOn");
      if (self.services.count > 0) {
        [self addAllServices];
        [self startAdvertising];
      }
      break;
    case CBPeripheralManagerStatePoweredOff:
      CBInfoLog(@"CBPeripheralManagerStatePoweredOff");
      [self stopAdvertising];
      break;
    case CBPeripheralManagerStateResetting:
      CBInfoLog(@"CBPeripheralManagerStateResetting");
      [self stopAdvertising];
      break;
    case CBPeripheralManagerStateUnauthorized:
      CBInfoLog(@"CBPeripheralManagerStateUnauthorized");
      [self stopAdvertising];
      break;
    case CBPeripheralManagerStateUnknown:
      CBInfoLog(@"CBPeripheralManagerStateUnknown");
      [self stopAdvertising];
      break;
    case CBPeripheralManagerStateUnsupported:
      CBInfoLog(@"CBPeripheralManagerStateUnsupported");
      [self stopAdvertising];
      break;
  }
}

- (void)peripheralManagerDidStartAdvertising:(CBPeripheralManager *)peripheral
                                       error:(nullable NSError *)error {
  [self threadSafetyCheck];
  NSTimeInterval responseTime = fabs([self.requestedAdvertisingStart timeIntervalSinceNow]);
  if (error) {
    CBErrorLog(@"Error advertising: %@", error);
    [self stopAdvertising];
    return;
  }
  CBDebugLog(@"Now advertising - %2.f msec to start", responseTime * 1000);
  self.advertisingState = AdvertisingStateAdvertising;
  if (self.advertisedServices.count > 1) {
    // Guarantee at least 100msec for battery
    NSTimeInterval howSoon = MAX(0.1, self.rotateAdDelay - responseTime);
    [self scheduleRotateAdTimer:howSoon];
  }
}

- (void)peripheralManager:(CBPeripheralManager *)peripheral
            didAddService:(CBService *)service
                    error:(nullable NSError *)error {
  [self threadSafetyCheck];
  CBAddServiceHandler handler = self.addServiceHandlers[service.UUID];
  self.addServiceHandlers[service.UUID] = nil;
  if (!handler) {
    CBDebugLog(@"Missing handler for adding service %@ with error %@", service, error);
    return;
  }
  handler(service.UUID, error);
}

- (void)peripheralManager:(CBPeripheralManager *)peripheral
                         central:(CBCentral *)central
    didSubscribeToCharacteristic:(CBCharacteristic *)characteristic {
  CBInfoLog(@"central %@ didSubscribeToCharacteristic %@", central, characteristic);
}

- (void)peripheralManager:(CBPeripheralManager *)peripheral
                             central:(CBCentral *)central
    didUnsubscribeFromCharacteristic:(CBCharacteristic *)characteristic {
  CBInfoLog(@"central %@ didUnsubscribeFromCharacteristic %@", central, characteristic);
}

- (void)peripheralManager:(CBPeripheralManager *)peripheral
    didReceiveReadRequest:(CBATTRequest *)request {
  CBDebugLog(@"didReceiveReadRequest %@", request);
  NSDictionary *characteristics = self.serviceCharacteristics[request.characteristic.service.UUID];
  if (!characteristics) {
    [peripheral respondToRequest:request withResult:CBATTErrorAttributeNotFound];
    return;
  }
  NSData *data = characteristics[request.characteristic.UUID];
  if (!data) {
    [peripheral respondToRequest:request withResult:CBATTErrorAttributeNotFound];
    return;
  }
  if (request.offset >= data.length) {
    [peripheral respondToRequest:request withResult:CBATTErrorInvalidOffset];
    return;
  }
  request.value = [data subdataWithRange:NSMakeRange(request.offset, data.length - request.offset)];
  [peripheral respondToRequest:request withResult:CBATTErrorSuccess];
}

- (void)peripheralManager:(CBPeripheralManager *)peripheral
  didReceiveWriteRequests:(NSArray<CBATTRequest *> *)requests {
  CBDebugLog(@"didReceiveWriteRequests %@", requests);
  for (CBATTRequest *request in requests) {
    [peripheral respondToRequest:request withResult:CBATTErrorWriteNotPermitted];
  }
}

- (void)threadSafetyCheck {
  assert(!strcmp(dispatch_queue_get_label(DISPATCH_CURRENT_QUEUE_LABEL),
                 dispatch_queue_get_label(self.queue)));
}

- (BOOL)isHardwarePoweredOn {
  return self.peripheral.state == CBPeripheralManagerStatePoweredOn;
}

- (NSString *)debugDescription {
  if (!self.queue) return @"[CBScanningDriver missing queue -- broken state]";
  __block NSString *out = nil;
  CBDispatchSync(self.queue, ^{
    NSString *state;
    switch (self.advertisingState) {
      case AdvertisingStateAdvertising:
        state = @"advertising";
        break;
      case AdvertisingStateNotAdvertising:
        state = @"not advertising";
        break;
      case AdvertisingStateStarting:
        state = @"starting";
        break;
    }
    out = [NSString stringWithFormat:@"[CBAdvertisingDriver isHardwareOn=%d advertisingState=%@ "
                                     @"rotateAd=%d serviceUuids=%@]",
                                     self.isHardwarePoweredOn, state, self.rotateAd,
                                     self.services.allKeys];
  });
  return out;
}

@end
