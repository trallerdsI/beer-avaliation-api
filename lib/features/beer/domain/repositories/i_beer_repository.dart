import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../entities/beer.dart';

abstract interface class IBeerRepository {
  Future<Either<Failure, List<Beer>>> getBeers();

  Future<Either<Failure, List<Beer>>> searchBeers({
    required String query,
    String? styleFilter,
  });

  Future<Either<Failure, Beer>> getBeerById(int id);
}
